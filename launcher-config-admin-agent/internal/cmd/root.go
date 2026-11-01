package cmd

import (
	"io"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executor/exec"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal"
	"github.com/spf13/pflag"
)

var (
	Version string
	values  *config.FlushCacheValues
)

func Execute() (err error, exitCode int) {
	var singleFs *cmd.SingleFlagSet
	values, singleFs = config.FlushCacheSingleFlagSet(Version, runRoot)
	return singleFs.Execute()
}

func runRoot(_ *pflag.FlagSet) (err error, exitCode int) {
	loggerInitFn(nil)
	if values.LogRoot != "" {
		initializeOrExitFn(values.LogRoot)
	}
	lock := newPidLockFn()
	if err = pidLockFn(lock); err != nil {
		cmdlog.Fail("Failed to lock pid file. Kill process config-admin-agent if it is running in your task manager.")
		loggerCloseFn()
		exitCode = common.ErrPidLock
		return
	}
	defer func() {
		loggerCloseFn()
		if r := recover(); r != nil {
			cmdlog.Fail("%s", r)
			cmdlog.Fault("%s", string(debug.Stack()))
			exitCode = common.ErrGeneral
		}
		_ = pidUnlockFn(lock)
	}()
	if !isAdminFn() {
		cmdlog.Fail("Program must be run as admin")
		exitCode = launcherCommon.ErrNotAdmin
		return
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			loggerCloseFn()
			_ = pidUnlockFn(lock)
			exitCode = common.ErrSignal
		}
	}()
	// Create the pipe before the flush below. The flush runs ipconfig, which on a
	// slow machine takes seconds, and the caller polls for this pipe for a couple
	// of seconds after launching us. Listening last meant that on such a machine
	// the caller gave up, killed an agent that was only still starting, and then
	// the agent carried on and sat in Accept forever. The pid file is already
	// written above, so the caller could see it alive the whole time.
	var listener net.Listener
	if listener, err = listenFn(); err != nil {
		exitCode = internal.ErrListen
		return
	}
	defer func() { _ = listener.Close() }()
	if values.IPs || values.Certs {
		if values.IPs {
			cmdlog.Step("Flushing IP cache...")
		}
		if values.Certs {
			cmdlog.Step("Flushing certificate cache...")
		}
		var result *exec.Result
		if buffErr := bufferFn("config-admin_flushCache", func(writer io.Writer) {
			_, result = runFlushCacheFn(values.IPs, values.Certs, values.LogRoot, writer, func(options *exec.Options) {
				if writer != nil {
					cmdlog.Println("run config admin flushCache", options.String())
				}
			})
		}); buffErr != nil {
			exitCode = common.ErrFileLog
			return
		}
		if !result.Success() {
			cmdlog.Fail("Failed to flush cache with exit code: %d", result.ExitCode)
			if result.Err != nil {
				cmdlog.Fault("%s", result.Err.Error())
			}
			exitCode = internal.ErrFlushCache
			return
		}
	}
	// Serve only now, so no command can race the flush, but the pipe has been
	// answerable since before it started.
	exitCode = serveFn(values.LogRoot, listener)
	return
}
