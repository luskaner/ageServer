package resolver

import (
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/appx"
)

func xboxPath(gameId string, battleServerPath string) (path string) {
	return locatablePath(func(gameId string) (game game.Locatable, ok bool) {
		return appx.NewGame(gameId)
	}, gameId, battleServerPath, "Xbox")
}

func resolveAutoPath(gameId string, battleServerPath string) (path string) {
	if path = steamPath(gameId, battleServerPath); path != "" {
		return
	}
	return xboxPath(gameId, battleServerPath)
}
