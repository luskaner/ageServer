package invitation

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type extendRequest struct {
	cancelRequest
	AdvertisementPassword string `schema:"gatheringpassword"`
}

func ExtendInvitation(w http.ResponseWriter, r *http.Request) {
	var q extendRequest
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	advertisements := models.G(r).Advertisements()
	adv, ok := advertisements.GetAdvertisement(q.AdvertisementId)
	if ok {
		var password string
		advertisements.WithReadLock(adv.GetId(), func() {
			password = adv.UnsafeGetPasswordValue()
		})
		ok = password == q.AdvertisementPassword
	}
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	inviter, peer, invitee, inviteeSession, ok := resolveInvitee(w, r, adv, q.UserId)
	if !ok {
		return
	}
	if !peer.Invite(invitee) {
		i.JSON(&w, i.A{0})
		return
	}
	if inviteeSession != nil {
		wss.SendOrStoreMessage(
			inviteeSession,
			"ExtendInvitationMessage",
			i.A{
				inviter.EncodeProfileInfo(inviteeSession.GetClientLibVersion()),
				q.AdvertisementId,
				q.AdvertisementPassword,
			},
		)
	}
	i.JSON(&w, i.A{0})
}
