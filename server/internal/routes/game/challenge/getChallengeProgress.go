package challenge

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/challenge/shared"
)

func GetChallengeProgress(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	i.JSON(&w, i.A{0, shared.GetChallengeProgressData(user)})
}
