package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/server/internal/cmd"
)

var version = "development"

func main() {
	// The capability has to be resolved before anything prints, and the file log
	// sink has to be told about the console before it decides what to prefix.
	ui.Initialize(os.Stdout, os.Environ())
	cmd.Version = version
	common.ChdirToExe()
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
