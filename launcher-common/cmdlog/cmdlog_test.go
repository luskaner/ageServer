package cmdlog_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// newSinks points the file log at one buffer and the console at another, so a test
// can see exactly what each one received.
func newSinks(t *testing.T) (fileLog, console *bytes.Buffer) {
	t.Helper()
	fileLog, console = &bytes.Buffer{}, &bytes.Buffer{}
	commonLogger.Initialize(fileLog)
	t.Cleanup(func() { commonLogger.Initialize(nil) })
	restoreOutput := ui.SetOutput(console)
	t.Cleanup(restoreOutput)
	// Pinned to ASCII with no colour, so the assertions are about the split between
	// the two sinks and not about what a terminal happens to support.
	restoreCapability := ui.SetCapability(ui.Capability{
		Profile: colorprofile.NoTTY, Tier: ui.TierASCII, Columns: 80,
	})
	t.Cleanup(restoreCapability)
	return fileLog, console
}

// A message goes to the file log as plain text and to the console as a marked line,
// and to each one exactly once.
//
// This is the bug the whole split exists to prevent, and it is easy to reintroduce
// by pointing commonLogger at os.Stdout: every message then appears twice, once
// with a timestamp and a |MAIN| prefix and once styled.
func TestEachSinkGetsItsOwnCopy(t *testing.T) {
	fileLog, console := newSinks(t)
	cmdlog.Fail("Failed to add hosts to hosts file.")

	if got := strings.Count(console.String(), "Failed to add hosts to hosts file."); got != 1 {
		t.Errorf("the console got the message %d times, want 1: %q", got, console.String())
	}
	if got := strings.Count(fileLog.String(), "Failed to add hosts to hosts file."); got != 1 {
		t.Errorf("the file log got the message %d times, want 1: %q", got, fileLog.String())
	}
	if !strings.Contains(console.String(), "[FAIL]") {
		t.Errorf("the console lost the marker: %q", console.String())
	}
	if strings.Contains(fileLog.String(), "[FAIL]") {
		t.Errorf("a marker reached the file log: %q", fileLog.String())
	}
	// The prefix and the timestamp belong to the file log and nowhere else.
	for _, decoration := range []string{"|MAIN|", "|main|"} {
		if strings.Contains(console.String(), decoration) {
			t.Errorf("the log decoration %q reached the console: %q", decoration, console.String())
		}
	}
}

// Every level has to reach both sinks: a renderer that only writes to one of them
// either loses the message or prints it twice.
func TestEveryLevelReachesBothSinks(t *testing.T) {
	for name, emit := range map[string]func(string, ...any){
		"Ok": cmdlog.Ok, "Fail": cmdlog.Fail, "Warn": cmdlog.Warn,
		"Info": cmdlog.Info, "Step": cmdlog.Step, "Detail": cmdlog.Detail,
		"Fault": cmdlog.Fault,
	} {
		t.Run(name, func(t *testing.T) {
			fileLog, console := newSinks(t)
			emit("Something happened.")
			if !strings.Contains(console.String(), "Something happened.") {
				t.Errorf("the console lost the message: %q", console.String())
			}
			if !strings.Contains(fileLog.String(), "Something happened.") {
				t.Errorf("the file log lost the message: %q", fileLog.String())
			}
		})
	}
}

// A percent sign in a message with no arguments must survive: these are format
// strings that happen to take none.
func TestMessagesWithoutArgumentsAreVerbatim(t *testing.T) {
	fileLog, console := newSinks(t)
	cmdlog.Warn("100%% of the ports are in use")
	if !strings.Contains(console.String(), "100%% of the ports are in use") {
		t.Errorf("got %q", console.String())
	}
	if !strings.Contains(fileLog.String(), "100%% of the ports are in use") {
		t.Errorf("got %q", fileLog.String())
	}
}
