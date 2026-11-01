package cmdUtils

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
)

var GameIds []string

func ParsedGameIds(gameIds *[]string) (games mapset.Set[string], err error) {
	if gameIds == nil {
		gameIds = &GameIds
	}
	if len(*gameIds) == 0 {
		// Clone: callers may mutate the result (e.g. Pop) and the exported
		// SupportedGames singleton must never be modified through it.
		games = game.SupportedGames.Clone()
	} else if !game.SupportedGames.IsSuperset(mapset.NewThreadUnsafeSet[string](*gameIds...)) {
		err = fmt.Errorf("game(s) not supported")
		return
	} else {
		games = mapset.NewSet[string](*gameIds...)
	}
	return
}
