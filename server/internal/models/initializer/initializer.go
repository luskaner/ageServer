package initializer

import (
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/age1"
	"github.com/luskaner/ageServer/server/internal/models/age2"
	"github.com/luskaner/ageServer/server/internal/models/age3"
	"github.com/luskaner/ageServer/server/internal/models/age4"
	"github.com/luskaner/ageServer/server/internal/models/athens"
)

var Games = map[string]models.Game{}

func InitializeGame(gameId string, configBattleServers []i.BattleServer) error {
	if err := models.InitializeBattleServers(gameId, configBattleServers); err != nil {
		return err
	}
	var g models.Game
	switch gameId {
	case game.AoE1:
		g = age1.CreateGame()
	case game.AoE2:
		g = age2.CreateGame()
	case game.AoE3:
		g = age3.CreateGame()
	case game.AoE4:
		g = age4.CreateGame()
	case game.AoM:
		g = athens.CreateGame()
	}
	Games[gameId] = g
	return nil
}
