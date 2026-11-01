package age3

import (
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/models"
)

func CreateGame() models.Game {
	return models.CreateMainGame(
		game.AoE3,
		&models.CreateMainGameOpts{
			BattleServer: &models.BattleServerOpts{
				Name:    "null",
				OobPort: true,
			},
		},
	)
}
