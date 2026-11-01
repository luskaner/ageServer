package leaderboard

import (
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/leaderboard/shared"
)

func GetStatGroupsByProfileIDs(w http.ResponseWriter, r *http.Request) {
	response := shared.GetStatGroups(
		r,
		r.URL.Query().Get("profileids"),
		true,
		models.G(r).Title() == game.AoE2,
	)
	i.JSON(&w, response)
}
