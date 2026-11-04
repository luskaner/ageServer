package achievement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// GetAvailableAchievements returns the available achievements list.
// Public endpoint without a game session.
func GetAvailableAchievements(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	i.JSON(&w, game.Resources().ArrayFiles()["achievements.json"])
}