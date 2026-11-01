package logger

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"

	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// newTestSinks points commonLogger at one buffer and the console at another, so
// a test can compare what each sink received.
func newTestSinks(t *testing.T) (fileLog, console *bytes.Buffer) {
	t.Helper()
	fileLog, console = &bytes.Buffer{}, &bytes.Buffer{}
	commonLogger.Initialize(fileLog)
	t.Cleanup(func() { commonLogger.Initialize(nil) })
	restore := ui.SetOutput(console)
	t.Cleanup(restore)
	// Pinned to ASCII so the markers in these assertions are the emoticons rather
	// than whatever the machine running the tests happens to support.
	capability := ui.SetCapability(ui.Capability{Profile: colorprofile.NoTTY, Tier: ui.TierASCII, Columns: 80})
	t.Cleanup(capability)
	return fileLog, console
}

// markedMessages is every renderer this package adds on top of the console
// plumbing.
func markedMessages() map[string]func(string, ...any) {
	return map[string]func(string, ...any){
		"Ok":     Ok,
		"Fail":   Fail,
		"Warn":   Warn,
		"Info":   Info,
		"Step":   Step,
		"Detail": Detail,
	}
}

// A marked message writes the same words to both sinks, and the file log gets
// neither the marker nor any decoration: that is what keeps launcher.txt
// grepeable.
func TestDualSinkReceivesTheSameWords(t *testing.T) {
	for name, emit := range markedMessages() {
		t.Run(name, func(t *testing.T) {
			fileLog, console := newTestSinks(t)
			emit("Successfully added host mappings")
			if got := fileLog.String(); strings.Contains(got, "[") {
				t.Errorf("a marker leaked into the file log: %q", got)
			}
			// The console must actually carry the marker, otherwise "the file log
			// has no marker" would pass for the wrong reason. Detail is the one
			// unmarked renderer: it belongs to the line above it.
			if name != "Detail" {
				if got := console.String(); !strings.Contains(got, "[") {
					t.Errorf("the console lost the marker: %q", got)
				}
			}
			if !strings.Contains(console.String(), "Successfully added host mappings") {
				t.Errorf("the console lost the message: %q", console.String())
			}
			if got := stripMarker(console.String()); !strings.HasSuffix(got, messageBody(fileLog.String())) {
				t.Errorf("the two sinks disagree\n  log: %q\n  console: %q", fileLog.String(), got)
			}
		})
	}
}

// No escape sequence and no non ASCII byte may ever reach the file log, whatever
// the console is capable of. go test runs with a pipe, so the capability is the
// conservative one here; the invariant is what matters, not the profile.
func TestFileLogNeverReceivesDecoration(t *testing.T) {
	for name, emit := range markedMessages() {
		t.Run(name, func(t *testing.T) {
			fileLog, _ := newTestSinks(t)
			emit("Successfully added host mappings for 192.168.1.50")
			got := fileLog.String()
			if strings.Contains(got, "\x1b") {
				t.Errorf("an escape sequence reached the file log: %q", got)
			}
			if !utf8.ValidString(got) {
				t.Errorf("the file log is not valid UTF-8: %q", got)
			}
			for i := 0; i < len(got); i++ {
				if b := got[i]; b > 0x7e && b != '\n' && b != '\t' {
					t.Errorf("byte %#x reached the file log: %q", b, got)
				}
			}
		})
	}
}

// The plain Printf/Println pair must keep reaching both sinks untouched.
func TestPlainPrintReachesBothSinks(t *testing.T) {
	fileLog, console := newTestSinks(t)
	Printf("Reverting changes made by a previous run\n")
	Println("Invalid game type")
	for _, want := range []string{"Reverting changes made by a previous run\n", "Invalid game type\n"} {
		if !strings.Contains(fileLog.String(), want) {
			t.Errorf("the file log is missing %q: %q", want, fileLog.String())
		}
		if !strings.Contains(console.String(), want) {
			t.Errorf("the console is missing %q: %q", want, console.String())
		}
	}
}

// Text is the substitution the file log gets. A message with a stray percent
// sign and no arguments must survive it unchanged.
func TestTextKeepsMessagesVerbatim(t *testing.T) {
	if got := text("100%% sure"); got != "100%% sure" {
		t.Errorf("got %q, want the text unchanged", got)
	}
	if got := text("copied %d of %d", 1, 2); got != "copied 1 of 2" {
		t.Errorf("got %q", got)
	}
}

// stripMarker removes the ASCII markers and the indentation the renderer adds,
// leaving the wording the file log is supposed to hold.
func stripMarker(s string) string {
	for _, marker := range []string{"[ OK ]", "[FAIL]", "[WARN]", "[INFO]", "[WAIT]"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n")
}

// messageBody returns the text of the last file log line without the timestamp
// and the |MAIN| prefix commonLogger adds when it is not writing to a console.
func messageBody(logged string) string {
	lines := strings.Split(strings.TrimSuffix(logged, "\n"), "\n")
	last := lines[len(lines)-1]
	if _, after, ok := strings.Cut(last, "|MAIN| "); ok {
		last = after
	}
	return last + "\n"
}
