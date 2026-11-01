package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config/internal/cmd"
)

var version = "development"

func main() {
	// commonLogger is both the console sink and, through its own route, the file
	// log sink, so the capability has to be resolved before Initialize keeps the
	// os.Stdout pointer comparison that decides its flags.
	ui.Initialize(os.Stdout, os.Environ())
	commonLogger.Initialize(os.Stdout)
	if executor.IsAdmin() {
		commonLogger.Println(ui.Warn("Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when it needs."))
	}
	common.ChdirToExe()
	cmd.Version = version
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
