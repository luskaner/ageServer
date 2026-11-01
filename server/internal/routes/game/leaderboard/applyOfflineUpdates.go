package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

func ApplyOfflineUpdates(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil && len(r.PostForm) > 0 {
		logger.Printf("leaderboard.applyOfflineUpdates: form=%s\n", r.PostForm.Encode())
	} else if err != nil {
		logger.Printf("leaderboard.applyOfflineUpdates: parse error: %s\n", err)
	} else {
		logger.Printf("leaderboard.applyOfflineUpdates: no form data\n")
	}
	i.JSON(&w, i.A{0, i.A{}, i.A{}})
}
