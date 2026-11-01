package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/game/leaderboard/shared"
)

func GetStatsForLeaderboardByProfileName(w http.ResponseWriter, r *http.Request) {
	response := shared.GetStatGroups(r, r.URL.Query().Get("profileids"), true, true)
	i.JSON(&w, response)
}
