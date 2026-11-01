package session

import (
	"context"
	"crypto/x509"
	"sync/atomic"
	"testing"

	"github.com/luskaner/ageServer/common"
	cmdServer "github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/fileLock"
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/executor/custom"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/spf13/pflag"
)

// A run that was asked to stop stops, and stopping it is not the same as
// failing.
//
// This is what a window's Stop button does. It used to be impossible to express:
// the only way to stop a run was to end the process, so a frontend that kept
// running had no way to interrupt a session without taking itself down with it.
func TestRunSessionStopsWhenCancelled(t *testing.T) {
	var mapHostsCalls, launchCalls atomic.Int32
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age2",
		isAdmin:       false,
		gameSupported: true,
		newPidLock:    func() fileLock.Locker { return &fakePidLocker{} },
		configStartServerFnVal: func(string, *pflag.FlagSet, *cmdServer.Values, bool) (int, string) {
			return common.ErrSuccess, "192.168.1.10"
		},
		serverReadCACertFnVal: func(string) *x509.Certificate { return nil },
		configMapHostsFnVal: func(string, string, bool, bool, bool) int {
			mapHostsCalls.Add(1)
			return common.ErrSuccess
		},
		configLaunchAgentAndGameFnVal: func(base.Executor, custom.Exec, []string, string, string, string) int {
			launchCalls.Add(1)
			return common.ErrSuccess
		},
	})
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, exitCode := Run(ctx, pflag.NewFlagSet("test", pflag.ContinueOnError)); exitCode != launcher.ErrCanceled {
		t.Fatalf("exit code = %d, want %d for a cancelled run", exitCode, launcher.ErrCanceled)
	}
	// The point of checking before each phase rather than inside them: a stop
	// that arrives before anything was changed must not go on to change it.
	if n := mapHostsCalls.Load(); n != 0 {
		t.Errorf("hosts were mapped %d times after being told to stop", n)
	}
	if n := launchCalls.Load(); n != 0 {
		t.Errorf("the game was launched %d times after being told to stop", n)
	}
}

// Cancelling a run that has not started yet costs nothing and undoes nothing,
// so it must not report a failure the user has to act on.
func TestRunSessionCancelledBeforeAnythingHappened(t *testing.T) {
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age2",
		isAdmin:       false,
		gameSupported: true,
		newPidLock:    func() fileLock.Locker { return &fakePidLocker{} },
	})
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, exitCode := Run(ctx, pflag.NewFlagSet("test", pflag.ContinueOnError)); exitCode != launcher.ErrCanceled {
		t.Fatalf("exit code = %d, want %d", exitCode, launcher.ErrCanceled)
	}
}

// A run nobody cancelled is unaffected by all of this.
func TestRunSessionWithoutCancellationIsUnchanged(t *testing.T) {
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age2",
		isAdmin:       false,
		gameSupported: true,
	})
	defer restore()

	_, exitCode := Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError))
	if exitCode == launcher.ErrCanceled {
		t.Error("a run nobody cancelled reported itself cancelled")
	}
}
