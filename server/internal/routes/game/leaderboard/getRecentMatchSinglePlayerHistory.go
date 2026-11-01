package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func GetRecentMatchSinglePlayerHistory(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{13, i.A{}})
}
