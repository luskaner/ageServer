package ui

import (
	"os"
	"strconv"
	"sync"
)

// The Windows Terminal taskbar and tab progress sequences.
//
// A terminal that understands OSC 9;4 draws the progress on the tab and, if the
// tab is in the taskbar, on the taskbar button itself: the button fills up, and it
// turns red or amber for an error or a warning. That indicator is the one place
// where a long step stays visible while the console scrolls, and unlike the
// in-place spinner it does not touch the text already on screen.
//
// The sequence is:
//
//	ESC ] 9 ; 4 ; <state> ; <progress> ST
//
// where state is one of the constants below and progress is 0 to 100. ST is the
// seven bit string terminator, ESC \. A terminal that does not know the sequence
// prints it verbatim, so it is only ever sent to a terminal known to understand
// it. See Progress.
const (
	progressRemove = 0 // clear the indicator, and the state it was showing
	progressSet    = 1 // show an indeterminate bar, with no percentage
	progressNormal = 2 // show a determinate bar at the given percentage
	progressError  = 3 // the operation failed
	progressWarn   = 4 // the operation finished with warnings
)

// progressST is the string terminator that closes the sequence.
const progressST = "\x1b\\"

// progressSequence builds one OSC 9;4 sequence. progress is clamped to 0..100
// because the indicator has nowhere to put 120, and a caller counting rounds
// should not have to know that.
func progressSequence(state, percent int) string {
	percent = min(max(percent, 0), 100)
	return "\x1b]9;4;" + strconv.Itoa(state) + ";" + strconv.Itoa(percent) + progressST
}

// Progress drives the terminal's own progress indicator.
//
// It is a no-op everywhere except Windows Terminal, and it never draws on the
// text: it only tells the shell how far along the work is. Every method is safe on
// a nil *Progress, so a caller does not have to know whether this terminal can
// show one.
type Progress struct {
	enabled bool
	mu      sync.Mutex
	// last is the last sequence written, so a repaint of the same value is skipped
	// and the indicator never flickers between two identical frames.
	last string
}

// BeginProgress takes the indicator for a bounded task. Always finish it with
// Done, Fail, Warn or Clear.
func BeginProgress() *Progress {
	p := &Progress{enabled: progressSupported()}
	if p.enabled {
		// State 1 is the honest one to start with: the total is not known until
		// the first round of discovery finishes, and an indeterminate bar says
		// "working" without claiming a number it cannot back up.
		p.write(progressRemove, 0)
		p.write(progressSet, 0)
	}
	return p
}

// progressSupported reports whether the terminal understands OSC 9;4.
//
// Windows Terminal is identified by WT_SESSION, which it sets in the environment
// of every shell it starts. That check is what keeps the sequence away from
// conhost, Windows Terminal's own predecessor, where the whole thing would be
// printed verbatim at the cursor.
//
// It also requires colour, because that is this package's existing answer to "is
// this terminal able to interpret an escape sequence at all": a run redirected to
// a file or a pipe is never a terminal, and `--output ascii` asks for plain text
// on purpose.
func progressSupported() bool {
	// No width means no console: the progress lives on a window that only exists
	// if there is one.
	if Current().ColumnLimit() <= 0 {
		return false
	}
	if !Current().ColorOk() {
		return false
	}
	if !isWindows {
		// Only Windows Terminal is assumed: OSC 9;4 is not universal, and a
		// sequence the terminal does not know is printed rather than ignored.
		return false
	}
	return envPresent(progressEnvironment(), "WT_SESSION")
}

// progressEnvironment is indirected so a test can pretend to be Windows Terminal
// without being on Windows.
var progressEnvironment = os.Environ

// Set reports a percentage of the work done.
func (p *Progress) Set(percent int) {
	if p == nil {
		return
	}
	p.write(progressNormal, percent)
}

// Fail leaves the indicator on the error state. It does not clear it: the point
// of a red bar is to still be there when the reader looks up.
func (p *Progress) Fail() {
	if p == nil {
		return
	}
	p.write(progressError, 100)
}

// Warn leaves the indicator on the warning state.
func (p *Progress) Warn() {
	if p == nil {
		return
	}
	p.write(progressWarn, 100)
}

// Done completes the indicator: it goes back to normal, at a full bar, and is then
// removed.
func (p *Progress) Done() {
	if p == nil {
		return
	}
	p.write(progressNormal, 100)
	p.Clear()
}

// Clear removes the indicator, which is what a taskbar button has to be told when
// the work stops without an outcome to report, and what every exit path needs so a
// crash does not leave a half filled button on the taskbar forever.
func (p *Progress) Clear() {
	if p == nil {
		return
	}
	p.write(progressRemove, 0)
}

// write emits one sequence, skipping a repeat of the last one.
func (p *Progress) write(state, percent int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.enabled {
		return
	}
	sequence := progressSequence(state, percent)
	if sequence == p.last {
		return
	}
	p.last = sequence
	write(sequence)
}

// ClearProgress removes the indicator without needing the tracker that created
// it. It belongs on the paths that exit without a verdict, such as a panic or a
// signal: a taskbar button left half filled after the program is gone is worse
// than no button at all.
func ClearProgress() {
	if !progressSupported() {
		return
	}
	write(progressSequence(progressRemove, 0))
}
