package invitation

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// resolveInvitee resolves the session's peer and the invited user for an
// invitation request, writing the error response on failure. The invitee's
// session may be absent (the user is not connected); that is not an error, so
// the caller gets a nil session and still records the invitation.
func resolveInvitee(w http.ResponseWriter, r *http.Request, adv models.Advertisement, inviteeID int32) (inviter models.User, peer models.Peer, invitee models.User, inviteeSession models.Session, ok bool) {
	game := models.G(r)
	inviter, _ = game.Users().GetUserById(models.SessionOrPanic(r).GetUserId())
	peer, ok = adv.GetPeers().Load(inviter.GetId())
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	invitee, ok = game.Users().GetUserById(inviteeID)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	inviteeSession, _ = game.Sessions().GetByUserId(invitee.GetId())
	ok = true
	return
}
