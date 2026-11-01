package party

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type advRequest struct {
	MatchID int32 `schema:"match_id"`
}

func UpdateHost(w http.ResponseWriter, r *http.Request) {
	var req advRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	game := models.G(r)
	advertisements := game.Advertisements()
	adv, ok := advertisements.GetAdvertisement(req.MatchID)
	if !ok {
		i.JSON(&w, i.A{2})
	} else {
		sess := models.SessionOrPanic(r)
		currentUserId := sess.GetUserId()
		peers := adv.GetPeers()
		if firstUserId, _, ok := peers.First(); !ok {
			i.JSON(&w, i.A{2})
			return
		} else if firstUserId != currentUserId {
			i.JSON(&w, i.A{2})
			return
		}
		advertisements.WithWriteLock(adv.GetId(), func() {
			hostId := adv.UnsafeGetHostId()
			if hostId == currentUserId {
				return
			}
			adv.UnsafeSetHostId(currentUserId)
		})
		i.JSON(&w, i.A{0})
	}
}
