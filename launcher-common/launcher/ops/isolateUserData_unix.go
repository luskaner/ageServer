//go:build !windows

package ops

import (
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/executor/steam"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine/crossover"
	wineExec "github.com/luskaner/ageServer/common/game/executor/wine"
	commonSteam "github.com/luskaner/ageServer/common/game/steam"
	wineSteam "github.com/luskaner/ageServer/common/game/wine/steam"
)

func (c *Config) wineIsolationPath(executer base.Executor) (path string) {
	switch executer.(type) {
	case *wine.Exec, *crossover.Exec:
		return wineSteam.OutputLauncherConfigHelper(
			executer.(wineExec.CustomExec),
			"userProfilePath",
			[]string{
				c.gameId,
			},
		)
	}
	return ""
}

func (c *Config) IsolationPath(executer base.Executor) (path string) {
	switch executer.(type) {
	case *steam.Exec:
		return commonSteam.UserProfilePath(c.gameId)
	default:
		return c.wineIsolationPath(executer)
	}
}
