package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/fileLock"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-agent/internal"
	"github.com/luskaner/ageServer/launcher-agent/internal/notify"
	"github.com/luskaner/ageServer/launcher-agent/internal/tray"
	"github.com/luskaner/ageServer/launcher-agent/internal/watch"
	"github.com/luskaner/ageServer/launcher-common/cmd/agent"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/spf13/pflag"
)

var Version string
var values *agent.Values

var (
	createLockFn   = func() fileLock.Locker { return &fileLock.PidLock{} }
	chdirToExeFn   = common.ChdirToExe
	initializeFn   = internal.Initialize
	watchFn        = watch.Watch
	signalNotifyFn = signal.Notify
	// startTrayFn is behind a var for the same reason as the rest: a test decides
	// whether a run has an icon instead of putting one on the desktop of whoever
	// is running the tests.
	startTrayFn = tray.Start
)

func Execute() (err error, exitCode int) {
	var singleFs *cmd.SingleFlagSet
	values, singleFs = agent.SingleFlagSet(Version, runRoot)
	return singleFs.Execute()
}

func runRoot(_ *pflag.FlagSet) (err error, exitCode int) {
	commonLogger.Initialize(os.Stdout)
	lock := createLockFn()
	if err = lock.Lock(); err != nil {
		cmdlog.Fail("Failed to lock pid file. Kill process agent if it is running in your task manager.")
		exitCode = common.ErrPidLock
		return
	}
	chdirToExeFn()
	if values.LogRoot != "" && values.BaseDataPath != "" {
		initializeFn(values.LogRoot)
	}
	var cleanupOnce sync.Once
	// Shared by the signal handler below and Watch, which run concurrently, so
	// the exit code goes through a synchronised holder rather than a bare *int.
	code := watch.NewExitCode()
	// The one way a run is asked to stop before it has finished on its own.
	// A terminate signal and the tray's Exit both cancel it and nothing else:
	// neither of them tears anything down, because the teardown has an order and
	// an exactly-once guarantee that belong to watch.Cleanup and to nobody else.
	ctx, stopRun := context.WithCancel(context.Background())
	defer stopRun()
	sigs := make(chan os.Signal, 1)
	signalNotifyFn(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			cmdlog.Step("Received terminate signal, shutting down...")
			code.Set(common.ErrSignal)
			stopRun()
		}
	}()
	// Started before the wait, not after: the whole point is to be on screen for
	// the wait, which is where the run spends its life. It reports what is
	// happening through internal.SetActivity, which the steps below publish to.
	systemTray := startTrayFn(values.GameId, stopRun)
	defer systemTray.Stop()
	watchFn(
		ctx,
		values,
		code,
		&cleanupOnce,
	)
	// A run that was stopped on purpose is recorded here rather than in the watch,
	// because the watch only knows that it was interrupted and an interrupted
	// wait is not a failed one. SetIfSuccess, and not Set, because the teardown
	// has already run by this point: a revert that failed is a worse thing to
	// report than the stop that led to it, and this must not paper over it.
	if ctx.Err() != nil {
		code.SetIfSuccess(internal.ErrStopped)
	}
	exitCode = code.Get()
	_ = lock.Unlock()
	// The session is over and this is the only moment left to tell the user so. The
	// terminal is a window nobody is looking at by now, and the answer they want is
	// the one they cannot ask for: whether the machine is clean again. All three
	// outcomes are announced, because "everything is back to normal" is the message
	// that lets them stop thinking about it, "something did not revert" is the one
	// that makes them go and read the logs, and "you stopped this" is the one that
	// stops a user who closed a game that is still open from looking for it.
	announce(*values.GameIdRef(), exitCode)
	return
}

// announce is the notification, behind a var so a test can observe it without a
// desktop.
var announce = func(game string, exitCode int) {
	var err error
	switch exitCode {
	case common.ErrSuccess:
		err = notify.SessionEnded(game)
	case internal.ErrStopped:
		err = notify.SessionStopped(game)
	default:
		err = notify.SessionFailed(game, exitCode)
	}
	// A notification that cannot be shown changes nothing about the run: the exit
	// code was already decided and the file log already has the reason. It is
	// recorded so that a user who saw no notification can find out why.
	switch {
	case err == nil:
	case notify.Unsupported(err):
		cmdlog.Detail("This system cannot show desktop notifications: %s", err)
	default:
		cmdlog.Detail("Could not show the desktop notification: %s", err)
	}
}
