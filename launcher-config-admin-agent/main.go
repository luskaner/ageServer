package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal/cmd"
)

var version = "development"

func main() {
	// The agent is started by the launcher and normally runs headless, but its
	// failures are read by a human, so it prints like everything else.
	cmdlog.Initialize()
	cmdlog.Banner(executables.LauncherConfigAdminAgent, version)
	cmd.Version = version
	common.ChdirToExe()
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
