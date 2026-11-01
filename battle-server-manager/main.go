package main

import (
	"battle-server-manager/internal/cmd"
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executables"
)

var version = "development"

func main() {
	cmdlog.Initialize()
	cmdlog.Banner(executables.BattleServerManager, version)
	cmd.Version = version
	common.ChdirToExe()
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
