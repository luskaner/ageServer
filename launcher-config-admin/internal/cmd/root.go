package cmd

import (
	"github.com/luskaner/ageServer/common/cmd"
)

var Version string
var rootFlagSet *cmd.RootFlagSet

func Execute() (err error, exitCode int) {
	rootFlagSet = cmd.NewRootFlagSet()
	rootFlagSet.RegisterCommand("setup", runSetUp)
	rootFlagSet.RegisterCommand("revert", runRevert)
	rootFlagSet.RegisterCommand("flushCache", runFlushCache)
	return rootFlagSet.Execute(Version)
}
