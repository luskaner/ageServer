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
	game := models.G(r)
	adv, ok := game.Advertisements().GetAdvertisement(q.AdvertisementId)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	peers := adv.GetPeers()
	var peer models.Peer
	sess := models.SessionOrPanic(r)
	u, _ := game.Users().GetUserById(sess.GetUserId())
	peer, ok = peers.Load(u.GetId())
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	var invitee models.User
	invitee, ok = game.Users().GetUserById(q.UserId)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	if !peer.Uninvite(invitee) {
		i.JSON(&w, i.A{0})
		return
	}
	var inviteeSession models.Session
	inviteeSession, ok = game.Sessions().GetByUserId(invitee.GetId())
	if ok {
		wss.SendOrStoreMessage(
			inviteeSession,
			"CancelInvitationMessage",
			i.A{
				u.EncodeProfileInfo(inviteeSession.GetClientLibVersion()),
				q.AdvertisementId,
			},
		)
	}
	i.JSON(&w, i.A{0})
}
