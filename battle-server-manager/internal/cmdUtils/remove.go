package cmdUtils

import (
	"battle-server-manager/internal"
	"os"
	"path/filepath"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/process"
)

var (
	parsedGameIdsFnRemoveAll = ParsedGameIds
	battleServerConfigsFn    = battleServer.Configs
	removeFnRemoveAll        = Remove
	findProcessFn            = process.FindProcess
	killProcFn               = process.KillProc
)

func Kill(config battleServer.Config) bool {
	proc, err := findProcessFn(int(config.PID))
	if err == nil && proc != nil {
		if err = killProcFn(proc); err == nil {
			cmdlog.Ok("Process still running, killed.")
			return true
		}
		cmdlog.Fail("Process still running, could not kill it: %s", err)
		return false
	}
	return true
}

func remove(gameId string, config battleServer.Config) bool {
	cmdlog.Step("Removing %s...", config.Region)
	_ = Kill(config)
	folder := battleServer.Folder(gameId)
	if f, err := os.Stat(folder); err != nil || !f.IsDir() {
		return false
	}
	fullPath := filepath.Join(folder, config.Path())
	if f, err := os.Stat(fullPath); err != nil || f.IsDir() {
		cmdlog.Fail("Failed with error: %s", err)
		return false
	}
	if err := os.Remove(fullPath); err != nil {
		cmdlog.Fail("Removing config file failed with error: %s", err)
		return false
	}
	cmdlog.Ok("Removed config file.")
	return true
}

func Remove(gameId string, configs []battleServer.Config, onlyInvalid bool) bool {
	var removedAny bool
	for _, config := range configs {
		var doRemove bool
		if onlyInvalid {
			if !config.Validate(false) {
				doRemove = true
			}
		} else {
			doRemove = true
		}
		if doRemove {
			removed := remove(gameId, config)
			removedAny = removedAny || removed
		}
	}
	return removedAny
}

func RemoveAll(onlyInvalid bool) (err error, exitCode int) {
	var games mapset.Set[string]
	games, err = parsedGameIdsFnRemoveAll(nil)
	if err != nil {
		cmdlog.Fail("%s", err.Error())
		exitCode = internal.ErrGames
		return
	}
	var configs []battleServer.Config
	for g := range games.Iter() {
		cmdlog.Section("Game " + g)
		configs, err = battleServerConfigsFn(g, false, false)
		if err != nil {
			cmdlog.Fault("%s", err)
			continue
		}
		if !removeFnRemoveAll(g, configs, onlyInvalid) {
			cmdlog.Info("No configuration needs it.")
		}
	}
	return
}
