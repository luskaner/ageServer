package ui

import (
	"strings"
	"sync"
	"time"
)

// spinnerFrames is the animation for a terminal that cannot draw anything but
// ASCII. Four frames is what a terminal can show without flicker.
var spinnerFrames = []string{"|", "/", "-", "\\"}

// spinnerFramesUnicode is the animation for a console with a UTF-8 code page: the
// braille spinner, which is a single cell, moves smoothly, and is in the font of
// every terminal that can be trusted with a symbol in the first place.
//
// It replaces what used to be a double width emoji marker rather than sitting next
// to one: an animated glyph that takes two cells would make the message jump
// sideways on every frame, which is the one thing an in place line must never do.
var spinnerFramesUnicode = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// frames returns the animation for the current tier.
func frames() []string {
	if Current().Tier == TierASCII {
		return spinnerFrames
	}
	return spinnerFramesUnicode
}

// spinnerInterval is how fast the frames change. Fast enough to read as motion,
// slow enough that the label stays legible and a redirected log never sees more
// than a handful of frames.
const spinnerInterval = 120 * time.Millisecond

// Spinner is an animated in-place line for the one kind of work that has nothing
// to print while it runs.
//
// It is the only thing in this package that redraws, and it is gated hard: it
// animates only on a console with a known width and colour, and everywhere else
// Start writes the plain one-shot line instead. That matters because the launcher
// can exit from a signal handler on another goroutine, and a redraw in flight
// would leave a half drawn line behind. Off a console there is nothing to redraw,
// so there is nothing to leave behind either.
//
// Every method is safe on a nil *Spinner, so a caller never has to ask whether the
// console could show one.
type Spinner struct {
	enabled bool
	label   string
	done    chan struct{}
	// erase is how many cells the live line occupies, including the label.
	erase int
	// mu guards the redraw and the ending. Ending takes it too, so a frame can
	// never land between the erase and the outcome: the line would come back to
	// life on top of the message that replaced it.
	mu       sync.Mutex
	finished bool
}

// Start begins an animated line with the given label. Always finish it with Done,
// Fail, Info or Stop, otherwise the line stays on screen.
func Start(label string) *Spinner {
	s := &Spinner{
		enabled: spinnersSupported(),
		label:   label,
		done:    make(chan struct{}),
	}
	if !s.enabled {
		// No console to redraw: one plain line is the whole feature.
		Println(Step("%s", label))
		return s
	}
	s.erase = lineWidth(label+"  ") + 2
	s.paint()
	go s.run()
	return s
}

// spinnersSupported reports whether a redraw is safe and visible.
//
// A known width means a console (a pipe or a file has none) and a profile above
// ASCII means it interprets the cursor moves. Below that the spinner degrades to
// a single line, which is what the TierASCII contract requires.
func spinnersSupported() bool {
	return Current().ColumnLimit() > 0 && Current().ColorOk()
}

// run redraws until the work finishes. The loop carries no counter: the frame is
// chosen from the clock, so a repaint that arrives late shows the frame that is due
// rather than one frame behind.
func (s *Spinner) run() {
	for {
		select {
		case <-s.done:
			return
		case <-time.After(spinnerInterval):
			s.paint()
		}
	}
}

func (s *Spinner) paint() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	set := frames()
	frame := set[time.Now().UnixNano()/int64(spinnerInterval)%int64(len(set))]
	write("\r" + stInfo.Render(frame+" ") + s.label)
}

// Stop erases the animated line without printing anything. Use it when the work
// produced no result worth a line of its own.
func (s *Spinner) Stop() { s.end("") }

// Done erases the animated line and replaces it with the outcome. A spinner that
// ended in anything but the success marker would leave the reader guessing whether
// the work passed.
func (s *Spinner) Done(f string, a ...any) { s.end(Ok(f, a...)) }

// Fail erases the animated line and replaces it with a failure.
func (s *Spinner) Fail(f string, a ...any) { s.end(Fail(f, a...)) }

// Info erases the animated line and replaces it with a note. Use it when the work
// finished without succeeding or failing, such as a search that found nothing.
func (s *Spinner) Info(f string, a ...any) { s.end(Info(f, a...)) }

// end is the single exit from the animated state: it stops the redraw, wipes the
// line and prints the outcome, in one atomic step.
//
// All three under one lock is the point. Wiping and printing separately lets a
// repaint that was already awake put the spinner back on top of the outcome, and
// on a shared console two writers at once is how a line ends up interleaved.
func (s *Spinner) end(outcome string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	close(s.done)
	if s.enabled {
		// The width of the erase is the widest the line ever got, which is fixed at
		// Start because the label does not change.
		write("\r" + strings.Repeat(" ", s.erase) + "\r")
	}
	if outcome != "" {
		Println(outcome)
	}
}
