package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher/internal/cmd"
)

var version = "development"

func main() {
	// The console capability has to be resolved before anything prints, and
	// before commonLogger.Initialize, which compares the writer against
	// os.Stdout by pointer to decide its flags.
	ui.Initialize(os.Stdout, os.Environ())
	commonLogger.Initialize(nil)
	cmd.Version = version
	common.ChdirToExe()
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
