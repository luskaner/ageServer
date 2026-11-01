package cmd

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/fileLock"
	"github.com/luskaner/ageServer/launcher-agent/internal"
	"github.com/luskaner/ageServer/launcher-agent/internal/tray"
	"github.com/luskaner/ageServer/launcher-agent/internal/watch"
	"github.com/luskaner/ageServer/launcher-common/cmd/agent"
)

// fakeLocker stands in for the pid lock: the test is about what the run announces,
// and a real lock would collide with an agent that happens to be running.
type fakeLocker struct{}

func (fakeLocker) Lock() error   { return nil }
func (fakeLocker) Unlock() error { return nil }

// overrides swaps every side effect runRoot reaches out for, so a test decides the
// outcome instead of waiting a minute for a process that never starts.
func overrides(t *testing.T, gameID string, exitCode int) (announced func() (string, int, int)) {
	t.Helper()
origValues, origLock, origChdir := values, createLockFn, chdirToExeFn
	origInit, origWatch, origNotify := initializeFn, watchFn, signalNotifyFn
	origAnnounce := announce
	origTray := startTrayFn

	values = &agent.Values{
		GameIdValues:  &cmd.GameIdValues{GameId: gameID},
		LogRootValues: &cmd.LogRootValues{},
	}
	createLockFn = func() fileLock.Locker { return fakeLocker{} }
	chdirToExeFn = func() {}
	// A nil log root already skips the real initialise, but the function is
	// replaced anyway so a change to that condition cannot start writing files.
	initializeFn = func(string) {}
	signalNotifyFn = func(chan<- os.Signal, ...os.Signal) {}
	watchFn = func(context.Context, *agent.Values, *watch.ExitCode, *sync.Once) {}
	startTrayFn = func(string, func()) *tray.Tray { return tray.Start("age2", func() {}) }

	var gotGame string
	var gotCode, calls int
	announce = func(game string, code int) {
		calls++
		gotGame, gotCode = game, code
	}
	// The exit code has to be produced by the watch, because that is where it comes
	// from in a real run: the caller reads it back after the cleanup.
	watchFn = func(ctx context.Context, _ *agent.Values, code *watch.ExitCode, _ *sync.Once) {
		if exitCode != common.ErrSuccess {
			code.Set(exitCode)
		}
	}

	t.Cleanup(func() {
		values, createLockFn, chdirToExeFn = origValues, origLock, origChdir
		initializeFn, watchFn, signalNotifyFn = origInit, origWatch, origNotify
		announce = origAnnounce
		startTrayFn = origTray
	})
	return func() (string, int, int) { return gotGame, gotCode, calls }
}

// A run announces its outcome, both ways.
//
// The agent outlives the launcher and prints to a console nobody is watching, so
// the notification is the only thing the user sees when the session ends. Missing
// it on the failure path would leave them with a machine they cannot account for
// and no signal that anything went wrong.
func TestRunRootAnnouncesBothOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exitCode int
	}{
		{"clean exit", common.ErrSuccess},
		{"failed exit", 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			announced := overrides(t, "age2", tc.exitCode)
			if _, exitCode := runRoot(nil); exitCode != tc.exitCode {
				t.Fatalf("exit code = %d, want %d", exitCode, tc.exitCode)
			}
			gotGame, gotCode, calls := announced()
			if calls != 1 {
				t.Fatalf("the outcome was announced %d times, want exactly 1", calls)
			}
			if gotCode != tc.exitCode {
				t.Errorf("announced exit code %d, want %d", gotCode, tc.exitCode)
			}
			if gotGame != "age2" {
				t.Errorf("announced game %q, want %q", gotGame, "age2")
			}
		})
	}
}

// The tray's Exit is a stop the user asked for, and it has to end up as one.
//
// This is the whole feature in one test: a run that is waiting for a game that is
// still open, the user clicking Exit, and the agent leaving with a code that says
// it was stopped. It goes through runRoot rather than through watch on purpose —
// the code is recorded by the caller, because only the caller can tell a stop
// apart from a teardown that reported an error.
func TestRunRootReportsATrayExitAsStopped(t *testing.T) {
	announced := overrides(t, "age2", common.ErrSuccess)
	exitRequested := make(chan func(), 1)
	var starts int
	// The game takes a long time and the run waits it out, which is the whole
	// reason there is anything to stop.
	watchFn = func(ctx context.Context, _ *agent.Values, _ *watch.ExitCode, _ *sync.Once) {
		<-ctx.Done()
	}
	startTrayFn = func(gameID string, onExit func()) *tray.Tray {
		starts++
		if gameID != "age2" {
			t.Errorf("the tray was given game %q, want %q", gameID, "age2")
		}
		exitRequested <- onExit
		return tray.Start("age2", onExit)
	}

	done := make(chan int, 1)
	go func() {
		_, exitCode := runRoot(nil)
		done <- exitCode
	}()

	// The click.
	(<-exitRequested)()

	select {
	case exitCode := <-done:
		if exitCode != internal.ErrStopped {
			t.Errorf("exit code = %d, want %d (ErrStopped)", exitCode, internal.ErrStopped)
		}
		if _, announcedCode, calls := announced(); calls != 1 || announcedCode != internal.ErrStopped {
			t.Errorf("announced %d, %d times; want %d, once", announcedCode, calls, internal.ErrStopped)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the run did not end after the tray's Exit")
	}
	if starts != 1 {
		t.Errorf("the tray was started %d times, want exactly 1", starts)
	}
}
