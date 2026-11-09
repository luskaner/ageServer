package resolver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/game"
)

var (
	battleServerResolvePath        = battleServer.ResolvePath
	battleServerResolvePathPriority = battleServer.ResolvePathPriority
	resolveAutoPathFn              = resolveAutoPath
	validPathFn                    = validPath
)

func validPath(path string) bool {
	if f, localErr := os.Stat(path); localErr == nil && !f.IsDir() {
		return true
	}
	return false
}

func locatablePath(locFn func(gameId string) (game game.Locatable, ok bool), gameId string, battleServerPath string, name string) (path string) {
	if locatable, ok := locFn(gameId); ok {
		if folder := locatable.Path(); folder != "" {
			tmpPath := filepath.Join(folder, battleServerPath)
			if validPathFn(tmpPath) {
				path = tmpPath
				cmdlog.Detail("Found in %s", name)
			}
		}
	}
	return
}

func ResolvePath(gameId string, executablePath string) (resolvedPath string, err error) {
	if executablePath == "auto" {
		cmdlog.Step("Auto resolving executable path...")
		if resolvedPath, err = doResolveAutoPath(gameId); err != nil {
			err = fmt.Errorf("auto resolution failed: %w", err)
		}
		return
	}
	var pathErr error
	var path string
	_, path, pathErr = common.ParsePath(common.EnhancedViperStringToStringSlice(executablePath), nil)
	if pathErr != nil {
		err = fmt.Errorf("invalid battle server executable path")
		return
	}
	if validPathFn(path) {
		resolvedPath = path
	} else {
		err = fmt.Errorf("invalid battle server executable path")
	}
	return
}

func doResolveAutoPath(gameId string) (resolvedPath string, err error) {
	// TODO: Review if AoE: DE and AoE III: DE can also use AoE II: DE
	// The Battle Server for AoM is buggy and the only one working is the AoE II one.
	// AoE IV does not have one.
	if gameId == game.AoE4 || gameId == game.AoM {
		gameId = game.AoE2
	}

	// Get the priority order of game IDs to try for battle server resolution
	priorityGameIds := battleServerResolvePathPriority(gameId)

	for _, priorityGameId := range priorityGameIds {
		var suffixPath string
		var ok bool
		if ok, suffixPath = battleServerResolvePath(priorityGameId); !ok {
			continue
		}
		path := resolveAutoPathFn(gameId, suffixPath)
		if path != "" && validPathFn(path) {
			// Warn when using a fallback battle server from another game
			if priorityGameId != gameId {
				switch gameId {
				case game.AoE1:
					if priorityGameId == game.AoE2 || priorityGameId == game.AoE3 {
						cmdlog.Warn("AoE1 is using %s's BattleServer.exe (fallback) - it may not work correctly", priorityGameId)
					}
				case game.AoE3:
					// AoE2 is the primary for AoE3, only warn for AoE3's own or AoE1
					if priorityGameId == game.AoE3 || priorityGameId == game.AoE1 {
						cmdlog.Warn("AoE3 is using %s's BattleServer.exe (fallback) - it may not work correctly", priorityGameId)
					}
				}
			}
			resolvedPath = path
			return
		}
	}

	err = fmt.Errorf("could not find battle server executable")
	return
}
