package age2

import (
	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/models"
)

func CreateGame() models.Game {
	return models.CreateMainGame(
		game.AoE2,
		&models.CreateMainGameOpts{
			Resources: &models.ResourcesOpts{
				KeyedFilenames: mapset.NewThreadUnsafeSet[string]("itemBundleItems.json", "itemDefinitions.json"),
			},
		},
	)
}
