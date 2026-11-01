package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
)

// Off a console there is nothing to redraw, so Start must degrade to a single
// plain line and never emit a carriage return. This is what keeps the TierASCII
// and redirected contracts intact.
func TestSpinnerIsOneLineWithoutAConsole(t *testing.T) {
	for _, columns := range []int{0, 80} {
		got := render(t, at(colorprofile.NoTTY, TierASCII, columns), func() {
			s := Start("Looking for the game")
			s.Done("Game found on Steam")
		})
		if strings.Contains(got, "\r") {
			t.Errorf("columns %d: a carriage return reached the output: %q", columns, got)
		}
		lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("columns %d: want the start line and the result, got %d:\n%q", columns, len(lines), got)
		}
		if !strings.Contains(lines[0], "Looking for the game") {
			t.Errorf("columns %d: the label is missing:\n%s", columns, got)
		}
		if !strings.Contains(lines[1], "Game found on Steam") {
			t.Errorf("columns %d: the result is missing:\n%s", columns, got)
		}
		if !AllASCII(got) {
			t.Errorf("columns %d: a non ASCII byte got out:\n%q", columns, got)
		}
	}
}

// On a console the line is redrawn in place and then erased, so the result is
// the only thing left on that row.
func TestSpinnerAnimatesAndErasesOnAConsole(t *testing.T) {
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 60))
	defer swap()
	s := Start("Looking for the game")
	time.Sleep(2 * spinnerInterval)
	s.Done("Game found on Steam")
	got := out.String()
	if !strings.Contains(got, "\r") {
		t.Errorf("a console must see the redraw:\n%q", got)
	}
	final := got[strings.LastIndex(got, "\r")+1:]
	// The outcome replaces the spinner on the same row, and it is the success
	// marker: an animation that ends in anything else would leave the reader
	// guessing whether the work passed.
	if !strings.Contains(StripANSI(final), G(GlyphSuccess)+" Game found on Steam") {
		t.Errorf("the last line written must be the outcome, got %q", final)
	}
	// Every redraw is one whole frame followed by the label. The two writes that
	// are not a redraw are the erase, which is spaces, and the outcome, which has
	// its own text.
	for _, frame := range strings.Split(got, "\r")[1:] {
		plain := StripANSI(frame)
		if strings.TrimSpace(plain) == "" || strings.Contains(plain, "Game found on Steam") {
			continue
		}
		if !strings.Contains(plain, "Looking for the game") {
			t.Errorf("an animation frame must carry the label, got %q", frame)
		}
	}
}

// Stop erases without printing, which is what a caller wants when the work
// produced nothing worth a line.
func TestSpinnerStopErasesWithoutPrinting(t *testing.T) {
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 60))
	defer swap()
	s := Start("Working")
	s.Stop()
	if got := out.String(); strings.Count(got, "\n") != 0 {
		t.Errorf("Stop must not leave a line behind: %q", got)
	}
}

// Every method has to be safe on a nil tracker, so a caller never has to ask
// whether the console could show a spinner.
func TestSpinnerNilIsSafe(t *testing.T) {
	var s *Spinner
	s.Stop()
	s.Done("done")
	s.Fail("failed")
	s.Info("nothing found")
}

// Info is the outcome of work that neither succeeded nor failed, such as a
// search that came back empty. It has to leave a line behind, unlike Stop.
func TestSpinnerInfoLeavesItsLine(t *testing.T) {
	for _, columns := range []int{0, 80} {
		got := render(t, at(colorprofile.NoTTY, TierASCII, columns), func() {
			s := Start("Looking for servers...")
			s.Info("No servers found.")
		})
		lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
		if len(lines) != 2 || !strings.Contains(lines[1], "No servers found.") {
			t.Errorf("columns %d: the note did not survive, got:\n%q", columns, got)
		}
	}
}

func TestSpinnerFramesAreASCII(t *testing.T) {
	for _, frame := range spinnerFrames {
		if !asciiOnly(frame) || frame == "" {
			t.Errorf("frame %q must be printable ASCII", frame)
		}
	}
}
