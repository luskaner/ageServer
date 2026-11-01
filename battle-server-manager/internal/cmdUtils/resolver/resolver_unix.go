//go:build !windows

package resolver

import (
	"fmt"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine"
	"github.com/luskaner/ageServer/common/game/executor/steam/wine/crossover"
)

type LocatableString interface {
	game.Locatable
	fmt.Stringer
}

func steamCrossOverPath(gameId string, battleServerPath string) (path string) {
	if ex, ok := crossover.NewExec(gameId); ok {
		path = winePath(ex, gameId, battleServerPath)
	}
	return
}

func steamWinePath(gameId string, battleServerPath string) (path string) {
	if ex, ok := wine.NewExec(gameId); ok {
		path = winePath(ex, gameId, battleServerPath)
	}
	return
}

func winePath(loc LocatableString, gameId string, battleServerPath string) (path string) {
	return locatablePath(func(gameId string) (game game.Locatable, ok bool) {
		game = loc
		ok = true
		return
	}, gameId, battleServerPath, loc.String())
}
