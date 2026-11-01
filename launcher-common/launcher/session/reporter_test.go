package session

import (
	"context"
	"fmt"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/spf13/pflag"
)

// installReporter points the session at r for the duration of a test and puts
// back whatever was there before.
func installReporter(t *testing.T, r launcher.Reporter) {
	t.Helper()
	orig := setup
	t.Cleanup(func() { setup = orig })
	Configure(Setup{Report: r})
}

// recordingReporter keeps what the run told it, so a test can assert on what a
// run said instead of on where it printed it.
type recordingReporter struct {
	launcher.Discard
	steps  []string
	fails  []string
	faults []string
}

func (r *recordingReporter) Step(format string, a ...any) {
	r.steps = append(r.steps, fmt.Sprintf(format, a...))
}

func (r *recordingReporter) Fail(format string, a ...any) {
	r.fails = append(r.fails, fmt.Sprintf(format, a...))
}

func (r *recordingReporter) Fault(format string, a ...any) {
	r.faults = append(r.faults, fmt.Sprintf(format, a...))
}

// The run hands its Reporter to the shared configuration.
//
// This is the seam a graphical frontend hooks. The console installes one that
// writes to the terminal and the log file; a window installs one that writes to
// an event stream, and then reads the whole run without owning the terminal. It
// used to be impossible, because the shared operations reported to a
// package-level console logger, so the only way to see a run was to be the
// terminal it printed to.
func TestRunRootInstallsItsReporterOnTheSharedConfig(t *testing.T) {
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age2",
		isAdmin:       false,
		gameSupported: true,
	})
	defer restore()

	rec := &recordingReporter{}
	installReporter(t, rec)

	// Deliberately a run that goes nowhere: the exit code is not what this is
	// about. What matters is that a shared Config was handed the Reporter
	// before anything asked it to do anything.
	if _, _ = Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError)); false {
		t.Fatal("unreachable")
	}
	if config.Report != launcher.Reporter(rec) {
		t.Fatalf("the shared config reports to %T, want the reporter the run installed", config.Report)
	}
}

// A Reporter that reports nothing must not be able to change what the run does.
// The exit code is decided by the run, not by whether anyone was listening.
func TestReporterCannotChangeTheOutcome(t *testing.T) {
	codes := make([]int, 0, 2)
	for _, r := range []launcher.Reporter{launcher.Discard{}, &recordingReporter{}} {
		restore := applyOverrides(t, runRootOverrides{
			gameId:        "age2",
			isAdmin:       false,
			gameSupported: true,
			cfg:           invalidServerStartConfig,
		})
		installReporter(t, r)
		_, exitCode := Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError))
		restore()
		codes = append(codes, exitCode)
	}
	if codes[0] != codes[1] {
		t.Fatalf("the exit code depends on who was listening: %d vs %d", codes[0], codes[1])
	}
	if codes[0] == common.ErrSuccess {
		t.Fatalf("expected the run to be rejected, got success")
	}
	if codes[0] != launcher.ErrInvalidServerStart {
		t.Fatalf("exit code = %d, want %d", codes[0], launcher.ErrInvalidServerStart)
	}
}

// invalidServerStart is a configuration that cannot produce a working run, so
// the run is rejected before anything is started.
func invalidServerStartConfig() *launcher.Configuration {
	c := validLauncherConfig()
	c.Server.Start = "maybe"
	return c
}
