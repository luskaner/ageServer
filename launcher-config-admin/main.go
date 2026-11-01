package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/logger"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal/cmd"
)

var version = "development"

func main() {
	ui.Initialize(os.Stdout, os.Environ())
	commonLogger.Initialize(os.Stdout)
	if !executor.IsAdmin() {
		commonLogger.Println(ui.Fail("This program must be run as an administrator"))
		os.Exit(launcherCommon.ErrNotAdmin)
	}
	common.ChdirToExe()
	cmd.Version = version
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
