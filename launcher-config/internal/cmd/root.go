package cmd

import (
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
)

var Version string
var path *commonUserData.Path
var rootFlagSet *commonCmd.RootFlagSet

func Execute() (err error, exitCode int) {
	rootFlagSet = commonCmd.NewRootFlagSet()
	rootFlagSet.RegisterCommand("setup", runSetUp)
	rootFlagSet.RegisterCommand("revert", runRevert)
	rootFlagSet.RegisterCommand("flushCache", runFlushCache)
	rootFlagSet.RegisterCommand("stopAgent", runStopAgent)
	return rootFlagSet.Execute(Version)
}
