package executor

import (
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/executor/custom"
	"github.com/luskaner/ageServer/common/game/executor/steam"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine/crossover"
)

func MakeExec(gameId string, executable string) base.Executor {
	if executable != "auto" {
		switch executable {
		case "steam":
			if executor, ok := steam.NewExec(gameId); ok {
				return executor
			}
		case "steam_crossover":
			if executor, ok := crossover.NewExec(gameId); ok {
				return executor
			}
		case "steam_wine":
			if executor, ok := wine.NewExec(gameId); ok {
				return executor
			}
		default:
			return custom.Exec{Executable: executable}
		}
		return nil
	}
	if executor, ok := steam.NewExec(gameId); ok {
		return executor
	}
	if executor, ok := crossover.NewExec(gameId); ok {
		return executor
	}
	if executor, ok := wine.NewExec(gameId); ok {
		return executor
	}
	return nil
}
