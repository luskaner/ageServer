package invitation

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/invitation/shared"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type cancelRequest struct {
	shared.Request
	UserId int32 `schema:"inviteeid"`
}

func CancelInvitation(w http.ResponseWriter, r *http.Request) {
	var q cancelRequest
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	adv, ok := models.G(r).Advertisements().GetAdvertisement(q.AdvertisementId)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	inviter, peer, invitee, inviteeSession, ok := resolveInvitee(w, r, adv, q.UserId)
	if !ok {
		return
	}
	if !peer.Uninvite(invitee) {
		i.JSON(&w, i.A{0})
		return
	}
	if inviteeSession != nil {
		wss.SendOrStoreMessage(
			inviteeSession,
			"CancelInvitationMessage",
			i.A{
				inviter.EncodeProfileInfo(inviteeSession.GetClientLibVersion()),
				q.AdvertisementId,
			},
		)
	}
	i.JSON(&w, i.A{0})
}
