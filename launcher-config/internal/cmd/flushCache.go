package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	launcherCommonCmd "github.com/luskaner/ageServer/launcher-common/cmd/config"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config/internal"
)

func runFlushCache(args []string) (err error, exitCode int) {
	flushCacheValues, flags := launcherCommonCmd.FlushCacheFlagSet()
	if err = flags.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			exitCode = common.ErrSignal
		}
	}()
	if flushCacheValues.LogRoot != "" {
		initializeFn(flushCacheValues.LogRoot)
	}
	if flushCacheValues.IPs || flushCacheValues.Certs {
		if isAdminFn() {
			err, exitCode = runFlushCacheAdminFn(flushCacheValues.LogRoot, flushCacheValues.IPs, flushCacheValues.Certs)
			if err == nil && exitCode == common.ErrSuccess {
				commonLogger.Println(commonUi.Ok("Successfully ran config-admin"))
			} else {
				if err != nil {
					commonLogger.Println(commonUi.Fault("Received error:"))
					commonLogger.Println(commonUi.Detail("%s", err))
				}
				if exitCode != common.ErrSuccess {
					commonLogger.Println(commonUi.Fault("Received exit code:"))
					commonLogger.Println(commonUi.Detail("%d", exitCode))
				}
				exitCode = internal.ErrAdminSetup
			}
		} else {
			agentStarted := connectAgentFn() == nil
			if agentStarted {
				exitCode = internal.ErrAgentAlreadyStarted
				return
			}
			result := startAgentFn(flushCacheValues.IPs, flushCacheValues.Certs)
			if !result.Success() {
				commonLogger.Println(commonUi.Fail("Failed to start config-admin-agent"))
				if result != nil {
					if result.Err != nil {
						commonLogger.Println(commonUi.Detail("%s", result.Err))
					}
					if result.ExitCode != common.ErrSuccess {
						commonLogger.Println(commonUi.Detail("%d", result.ExitCode))
					}
				}
				exitCode = internal.ErrStartAgent
			} else {
				agentStarted = connectAgentRetriesFn()
				if !agentStarted {
					commonLogger.Println(commonUi.Fail("Failed to connect to config-admin-agent after starting it. Kill it using the task manager."))
					_ = stopAgentIfNeededFn()
					exitCode = internal.ErrStartAgentVerify
				}
			}
		}
	}
	return
}
