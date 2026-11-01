package cmd

import (
	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func runFlushCache(args []string) (err error, exitCode int) {
	values, fs := config.FlushCacheFlagSet()
	if err = fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	if values.LogRoot != "" {
		if initErr := initializeFn(values.LogRoot); initErr != nil {
			commonLogger.Println(commonUi.Fail("Failed to initialize file logging: %s", initErr))
		}
	}
	if values.Certs {
		if runtimeGOOS != "windows" {
			commonLogger.Println(commonUi.Step("Flushing Certs cache..."))
			if result := flushCertsFn(); !result.Success() {
				commonLogger.Println(commonUi.Fail("Failed to flush Certs cache"))
				if result.ExitCode != common.ErrSuccess {
					commonLogger.Println(commonUi.Fault("Exit code: %v", result.ExitCode))
				}
				if result.Err != nil {
					commonLogger.Println(commonUi.Fault("Error: %v", result.Err))
				}
				exitCode = internal.ErrFlushCacheCerts
			}
		}
	}
	if values.IPs {
		if result := flushDnsFn(); !result.Success() {
			commonLogger.Println(commonUi.Fail("Failed to flush DNS cache"))
			if result.ExitCode != common.ErrSuccess {
				commonLogger.Println(commonUi.Fault("Exit code: %v", result.ExitCode))
			}
			if result.Err != nil {
				commonLogger.Println(commonUi.Fault("Error: %v", result.Err))
			}
			if exitCode == internal.ErrFlushCacheCerts {
				exitCode = internal.ErrFlushCache
			} else {
				exitCode = internal.ErrFlushCacheDNS
			}
		}
	}
	return
}
