package hosts

import (
	"io"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func FlushDns() (result *exec.Result) {
	commonLogger.Println(commonUi.Step("Flushing DNS cache..."))
	options := exec.Options{
		File:        "dscacheutil",
		SpecialFile: true,
		ExitCode:    true,
		Wait:        true,
		Args:        []string{"-flushcache"},
	}
	var suffix string
	if internal.SetUp == nil {
		suffix = "_flushCache"
	} else if *internal.SetUp {
		suffix = "_setup"
	} else {
		suffix = "_revert"
	}
	if err := internal.Logger.Buffer("dscacheutil_flushcache"+suffix, func(writer io.Writer) {
		if writer != nil {
			options.Stdout = writer
			options.Stderr = writer
			commonLogger.Println(commonUi.Detail("run dscacheutil: %s", options.String()))
		}
		result = options.Exec()
	}); err != nil {
		result = &exec.Result{ExitCode: common.ErrFileLog, Err: err}
		return result
	}
	options = exec.Options{
		File:        "killall",
		SpecialFile: true,
		ExitCode:    true,
		Wait:        true,
		Args:        []string{"-HUP", "mDNSResponder"},
	}
	if err := internal.Logger.Buffer("killall_hup_mDNSResponder"+suffix, func(writer io.Writer) {
		if writer != nil {
			options.Stdout = writer
			options.Stderr = writer
			commonLogger.Println(commonUi.Detail("run killall: %s", options.String()))
		}
		result = options.Exec()
	}); err != nil {
		result = &exec.Result{ExitCode: common.ErrFileLog, Err: err}
	}
	return
}
