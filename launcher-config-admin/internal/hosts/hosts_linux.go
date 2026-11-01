package hosts

import (
	"io"

	"github.com/hairyhenderson/go-which"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func FlushDns() (result *exec.Result) {
	path := which.Which("resolvectl")
	if path != "" {
		commonLogger.Println(commonUi.Step("Flushing DNS cache..."))
		options := exec.Options{File: "systemctl", SpecialFile: true, UseWorkingPath: true, ExitCode: true, Wait: true, Args: []string{"restart", "systemd-resolved"}}
		var suffix string
		if internal.SetUp == nil {
			suffix = "_flushCache"
		} else if *internal.SetUp {
			suffix = "_setup"
		} else {
			suffix = "_revert"
		}
		if err := internal.Logger.Buffer("resolvectl_flushcache"+suffix, func(writer io.Writer) {
			if writer != nil {
				options.Stdout = writer
				options.Stderr = writer
				commonLogger.Println(commonUi.Detail("run resolvectl: %s", options.String()))
			}
			result = options.Exec()
		}); err != nil {
			result = &exec.Result{
				ExitCode: common.ErrFileLog,
				Err:      err,
			}
		}
		return
	}
	// Some systems do not have, assume it is not needed
	return exec.ResultSuccess
}
