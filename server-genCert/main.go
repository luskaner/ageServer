package main

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/server-genCert/internal/cmd"
)

var version = "development"

func main() {
	cmdlog.Initialize()
	cmdlog.Banner(executables.ServerGenCert, version)
	cmd.Version = version
	common.ChdirToExe()
	err, exitCode := cmd.Execute()
	if err != nil {
		fmt.Print(err)
	}
	os.Exit(exitCode)
}
