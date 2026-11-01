package advertisement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func getAdvResp(errorCode int, advs i.A) i.A {
	return i.A{
		errorCode, // resultCode
		advs,      // advertisements
	}
}

type getAdvertisementsRequest struct {
	MatchIDs i.Json[[]int32] `schema:"match_ids"`
}

func GetAdvertisements(w http.ResponseWriter, r *http.Request) {
	var req getAdvertisementsRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	game := models.G(r)
	title := game.Title()
	sess := models.SessionOrPanic(r)
	advertisements := game.Advertisements()
	if len(req.MatchIDs.Data) > 0 {
		matchIDs := req.MatchIDs.Data
		advs := advertisements.LockedFindAdvertisementsEncoded(title, sess.GetClientLibVersion(), 0, 0, true, func(adv models.Advertisement) bool {
			for _, id := range matchIDs {
				if adv.GetId() == id {
					return true
				}
			}
			return false
		})
		if advs == nil {
			i.JSON(&w, getAdvResp(0, i.A{}))
		} else {
			i.JSON(&w, getAdvResp(0, advs))
		}
		return
	}
	advs := advertisements.LockedFindAdvertisementsEncoded(title, sess.GetClientLibVersion(), 0, 0, false, func(models.Advertisement) bool { return true })
	if advs == nil {
		i.JSON(&w, getAdvResp(0, i.A{}))
	} else {
		i.JSON(&w, getAdvResp(0, advs))
	}
}
