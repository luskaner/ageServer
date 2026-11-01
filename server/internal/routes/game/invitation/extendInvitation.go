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
	game := models.G(r)
	advertisements := game.Advertisements()
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
	if !peer.Invite(invitee) {
		i.JSON(&w, i.A{0})
		return
	}
	var inviteeSession models.Session
	inviteeSession, ok = game.Sessions().GetByUserId(invitee.GetId())
	if ok {
		wss.SendOrStoreMessage(
			inviteeSession,
			"ExtendInvitationMessage",
			i.A{
				u.EncodeProfileInfo(inviteeSession.GetClientLibVersion()),
				q.AdvertisementId,
				q.AdvertisementPassword,
			},
		)
	}
	i.JSON(&w, i.A{0})
}
