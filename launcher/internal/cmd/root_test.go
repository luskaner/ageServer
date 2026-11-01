package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/launcher-common/launcher/ops/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher/session"
)

// The console's Reporter still reaches the terminal.
//
// The session reports through a Reporter now instead of through the logger, so
// this is where a dropped line would show up: an adapter that reported into the
// void would leave every test green and leave a user watching a terminal that
// says nothing while the launcher works.
//
// It is the shared one, not a private copy. The file log half of every line has to
// happen whichever frontend is listening, and a window that had to reimplement the
// dual sink to get it would eventually get one of the two halves subtly wrong.
func TestConsoleReporterStillReachesTheTerminal(t *testing.T) {
	out := captureStdout(t, func() {
		logger.Reporter{}.Step("Setting up...")
		logger.Reporter{}.Fail("Failed to lock pid file.")
	})
	if !strings.Contains(out, "Setting up...") {
		t.Errorf("the stage never reached the terminal, got %q", out)
	}
	if !strings.Contains(out, "Failed to lock pid file.") {
		t.Errorf("the failure never reached the terminal, got %q", out)
	}
}

// The console installs itself with the session, so the run it performs narrates
// through the console and not through whatever was left over from before.
func TestConsoleConfiguresItselfWithTheSession(t *testing.T) {
	orig := Version
	Version = "1.2.3"
	t.Cleanup(func() { Version = orig })

	configure()
	installed := session.Current()
	if installed.Version != "1.2.3" {
		t.Errorf("the session shows version %q, want the console's", installed.Version)
	}
	if installed.Report == nil {
		t.Fatal("the session was left with no reporter")
	}
	if _, ok := installed.Report.(logger.Reporter); !ok {
		t.Errorf("the session reports through %T, want the shared dual-sink logger", installed.Report)
	}
	if installed.NewDialog == nil {
		t.Error("the session was left with no way to ask anything")
	}

}

// The dialog backend the console offers is the console's own, so a run that asks
// something and gets nothing answers on the terminal rather than nowhere.
func TestConsoleOffersAWorkingDialog(t *testing.T) {
	configure()
	resolution := session.Current().NewDialog(false)
	if resolution.Dialog == nil {
		t.Fatal("no backend for ModeFalse")
	}
	if got := resolution.Dialog.Name(); got != "console" {
		t.Errorf("ModeFalse resolved to %q, want the console", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	return <-done
}
