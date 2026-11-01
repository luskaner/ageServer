package cmd

import (
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/fileLock"
)

var Version string
var rootFlagSet *cmd.RootFlagSet

type pidLocker interface {
	Lock() error
	Unlock() error
}

var (
	newPidLock     = func() pidLocker { return &fileLock.PidLock{} }
	newRootFlagSet = cmd.NewRootFlagSet
)

func Execute() (err error, exitCode int) {
	lock := newPidLock()
	if err = lock.Lock(); err != nil {
		cmdlog.Fail("Failed to lock pid file. Kill process battle-server-manager if it is running in your task manager.")
		cmdlog.Fault("%s", err.Error())
		exitCode = common.ErrPidLock
		return
	}
	defer func() {
		_ = lock.Unlock()
	}()
	rootFlagSet = newRootFlagSet()
	rootFlagSet.RegisterCommand("clean", runClean)
	rootFlagSet.RegisterCommand("remove", runRemove)
	rootFlagSet.RegisterCommand("remove-all", runRemoveAll)
	rootFlagSet.RegisterCommand("start", runStart)
	return rootFlagSet.Execute(Version)
}
