package main

import (
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server-replay/internal/cmd"
)

var version = "development"

func main() {
	cmd.Version = version
	common.ChdirToExe()
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
