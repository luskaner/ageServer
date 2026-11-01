package resolver

import (
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/steam"
)

func steamPath(gameId string, battleServerPath string) (path string) {
	return locatablePath(func(gameId string) (game game.Locatable, ok bool) {
		return steam.NewGame(gameId)
	}, gameId, battleServerPath, "Steam")
}
