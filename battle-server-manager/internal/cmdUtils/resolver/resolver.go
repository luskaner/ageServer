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
	battleServerResolvePath = battleServer.ResolvePath
	resolveAutoPathFn       = resolveAutoPath
	validPathFn             = validPath
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
	var suffixPath string
	var ok bool
	if ok, suffixPath = battleServerResolvePath(gameId); !ok {
		err = fmt.Errorf("could not find battle server executable")
		return
	}
	path := resolveAutoPathFn(gameId, suffixPath)
	if path == "" {
		err = fmt.Errorf("could not find battle server executable")
		return
	}
	if !validPathFn(path) {
		err = fmt.Errorf("could not find battle server executable")
		return
	}
	resolvedPath = path
	return
}
