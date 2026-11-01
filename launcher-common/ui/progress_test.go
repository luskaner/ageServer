package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// The exact bytes matter: this is a published sequence, and a terminal that
// receives a malformed one shows it instead of acting on it.
func TestProgressSequenceFormat(t *testing.T) {
	for _, tc := range []struct {
		state, percent int
		want           string
	}{
		{progressRemove, 0, "\x1b]9;4;0;0\x1b\\"},
		{progressSet, 0, "\x1b]9;4;1;0\x1b\\"},
		{progressNormal, 0, "\x1b]9;4;2;0\x1b\\"},
		{progressNormal, 50, "\x1b]9;4;2;50\x1b\\"},
		{progressError, 100, "\x1b]9;4;3;100\x1b\\"},
		{progressWarn, 100, "\x1b]9;4;4;100\x1b\\"},
		// Out of range values are clamped rather than sent: the indicator has
		// nowhere to put 120, and a caller counting rounds should not have to know.
		{progressNormal, 120, "\x1b]9;4;2;100\x1b\\"},
		{progressNormal, -5, "\x1b]9;4;2;0\x1b\\"},
	} {
		if got := progressSequence(tc.state, tc.percent); got != tc.want {
			t.Errorf("progressSequence(%d,%d) = %q, want %q", tc.state, tc.percent, got, tc.want)
		}
	}
}

// pretendWindowsTerminal makes the gate open on any host.
func pretendWindowsTerminal(t *testing.T) {
	t.Helper()
	prevEnv, prevOS := progressEnvironment, isWindows
	progressEnvironment = func() []string { return []string{"WT_SESSION=abc"} }
	isWindows = true
	t.Cleanup(func() { progressEnvironment, isWindows = prevEnv, prevOS })
}

// The whole lifecycle, as bytes. Begin shows an indeterminate bar, Set reports the
// percentage, Done completes it and removes it, and what is left is nothing for
// the terminal to keep drawing.
func TestProgressLifecycle(t *testing.T) {
	pretendWindowsTerminal(t)
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
	defer swap()

	p := BeginProgress()
	p.Set(0)
	p.Set(50)
	p.Set(100)
	p.Done()

	want := "\x1b]9;4;0;0\x1b\\" + // begin: clear anything stale first
		"\x1b]9;4;1;0\x1b\\" + // indeterminate, the total is not known yet
		"\x1b]9;4;2;0\x1b\\" +
		"\x1b]9;4;2;50\x1b\\" +
		"\x1b]9;4;2;100\x1b\\" +
		"\x1b]9;4;0;0\x1b\\" // done: 100 was already shown, so it just removes
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

// A bar that has to be red when the run failed, and gone when it did not.
func TestProgressFailureAndWarningStates(t *testing.T) {
	pretendWindowsTerminal(t)
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
	defer swap()

	failed := BeginProgress()
	failed.Fail()
	if got := out.String(); !strings.Contains(got, "\x1b]9;4;3;100\x1b\\") {
		t.Errorf("the error state was not sent: %q", got)
	}

	out.Reset()
	warned := BeginProgress()
	warned.Warn()
	if got := out.String(); !strings.Contains(got, "\x1b]9;4;4;100\x1b\\") {
		t.Errorf("the warning state was not sent: %q", got)
	}
}

// Clearing without a verdict is what a signal handler or a panic needs: a taskbar
// button left half filled after the program is gone is worse than no button.
func TestClearProgressRemovesTheIndicator(t *testing.T) {
	pretendWindowsTerminal(t)
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
	defer swap()

	ClearProgress()
	if got := out.String(); got != "\x1b]9;4;0;0\x1b\\" {
		t.Errorf("got %q, want the remove sequence", got)
	}
}

// Everything else: a terminal that does not know the sequence would print it at
// the cursor, so the gate has to hold.
func TestProgressIsSilentWhenUnsupported(t *testing.T) {
	t.Run("no WT_SESSION", func(t *testing.T) {
		prevEnv, prevOS := progressEnvironment, isWindows
		progressEnvironment = func() []string { return []string{"TERM=xterm"} }
		isWindows = true
		defer func() { progressEnvironment, isWindows = prevEnv, prevOS }()
		var out strings.Builder
		swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
		defer swap()
		BeginProgress().Set(50)
		ClearProgress()
		if out.Len() != 0 {
			t.Errorf("a terminal that does not know OSC 9;4 got %q", out.String())
		}
	})
	t.Run("no colour", func(t *testing.T) {
		pretendWindowsTerminal(t)
		for _, c := range []Capability{at(colorprofile.NoTTY, TierASCII, 80), {Profile: colorprofile.NoTTY}} {
			var out strings.Builder
			swap := setWriter(&out, c)
			func() {
				defer swap()
				BeginProgress().Set(50)
				ClearProgress()
			}()
			if out.Len() != 0 {
				t.Errorf("capability %+v got %q, want nothing", c, out.String())
			}
		}
	})
	t.Run("not a terminal", func(t *testing.T) {
		pretendWindowsTerminal(t)
		var out strings.Builder
		swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 0))
		defer swap()
		BeginProgress().Set(50)
		if out.Len() != 0 {
			t.Errorf("a pipe got %q", out.String())
		}
	})
}

func TestProgressNilIsSafe(t *testing.T) {
	var p *Progress
	p.Set(50)
	p.Done()
	p.Fail()
	p.Warn()
	p.Clear()
}

// The same percentage twice in a row is one frame, not two.
func TestProgressSkipsARepeatedValue(t *testing.T) {
	pretendWindowsTerminal(t)
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
	defer swap()

	p := BeginProgress()
	out.Reset()
	p.Set(25)
	p.Set(25)
	p.Set(25)
	if n := strings.Count(out.String(), "\x1b]9;4;2;25"); n != 1 {
		t.Errorf("the same value was sent %d times, want 1: %q", n, out.String())
	}
}

// Reports arrive from the discovery goroutines at the same time, and the indicator
// has to be one whole sequence at a time.
func TestProgressIsSafeFromManyGoroutines(t *testing.T) {
	pretendWindowsTerminal(t)
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierEmoji, 80))
	defer swap()

	p := BeginProgress()
	done := make(chan struct{})
	for i := range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range 25 {
				p.Set((i + j) % 101)
			}
		}()
	}
	for range 8 {
		<-done
	}
	p.Done()

	// Every write must be one complete sequence, so an odd number of ESC bytes is
	// proof that two of them interleaved.
	for _, sequence := range strings.Split(out.String(), "\x1b]9;4;")[1:] {
		if !strings.HasSuffix(sequence, "\x1b\\") {
			t.Fatalf("an interleaved write: %q", sequence)
		}
	}
}
