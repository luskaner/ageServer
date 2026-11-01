package hosts

import (
	"io"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func FlushDns() (result *exec.Result) {
	commonLogger.Println(commonUi.Step("Flushing DNS cache..."))
	options := exec.Options{File: "ipconfig", SpecialFile: true, UseWorkingPath: true, ExitCode: true, Wait: true, Args: []string{"/flushdns"}}
	var suffix string
	if internal.SetUp == nil {
		suffix = "_flushCache"
	} else if *internal.SetUp {
		suffix = "_setup"
	} else {
		suffix = "_revert"
	}
	if err := internal.Logger.Buffer("ipconfig_flushdns"+suffix, func(writer io.Writer) {
		if writer != nil {
			options.Stdout = writer
			options.Stderr = writer
			commonLogger.Println(commonUi.Detail("run ipconfig: %s", options.String()))
		}
		result = options.Exec()
		if result != nil && !result.Success() {
			commonLogger.Println(commonUi.Fail("Flushing DNS cache finished with exit code %d.", result.ExitCode))
			if result.Err != nil {
				commonLogger.Println(commonUi.Detail("Flush DNS error: %v", result.Err))
			}
		}
	}); err != nil {
		commonLogger.Println(commonUi.Fail("Failed to buffer flush DNS log: %v", err))
		result = &exec.Result{
			ExitCode: common.ErrFileLog,
			Err:      err,
		}
	}
	return
}
