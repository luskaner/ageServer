package achievement

import (
	"net/http"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
)

func GrantAchievement(w http.ResponseWriter, _ *http.Request) {
	// DO NOT ALLOW THE CLIENT TO CLAIM ACHIEVEMENTS
	i.JSON(&w,
		i.A{
			2,
			time.Now().UTC().Unix(),
		},
	)
}
