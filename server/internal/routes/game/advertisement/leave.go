package advertisement

import (
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"

	"net/http"
)

func Leave(w http.ResponseWriter, r *http.Request) {
	var q shared.AdvertisementId
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	game := models.G(r)
	advertisements := game.Advertisements()
	sess := models.SessionOrPanic(r)
	var success bool
	advertisements.WithWriteLock(q.AdvertisementId, func() {
		adv, ok := advertisements.GetAdvertisement(q.AdvertisementId)
		if !ok {
			return
		}
		success = advertisements.UnsafeRemovePeer(adv.GetId(), sess.GetUserId())
	})
	game.Matchmaking().RemoveUser(sess.GetUserId())
	if success {
		i.JSON(&w, i.A{0})
	} else {
		i.JSON(&w, i.A{2})
	}
}
