package party

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/party/shared"
)

func PeerUpdate(w http.ResponseWriter, r *http.Request) {
	// What about isNonParticipants[]? observers? ai players?
	parseError, advId, length, profileIds, raceIds, teamIds := shared.ParseParameters(r)
	if parseError {
		i.JSON(&w, i.A{2})
		return
	}
	sess := models.SessionOrPanic(r)
	game := models.G(r)
	gameUsers := game.Users()
	advertisements := game.Advertisements()
	currentUserId := sess.GetUserId()
	var ok bool
	advertisements.WithWriteLock(advId, func() {
		adv, exists := advertisements.GetAdvertisement(advId)
		if !exists {
			return
		}
		// Only the host can update peers
		if hostId := adv.UnsafeGetHostId(); hostId != currentUserId {
			return
		}
		advPeers := adv.GetPeers()
		var peers []models.Peer
		for j := range length {
			var u models.User
			u, ok = gameUsers.GetUserById(profileIds[j])
			if !ok {
				return
			}
			var peer models.Peer
			peer, ok = advPeers.Load(u.GetId())
			if !ok {
				return
			}
			peers = append(peers, peer)
		}
		for j, peer := range peers {
			peer.UpdateMutable(raceIds[j], teamIds[j])
		}
		ok = true
	})
	if ok {
		i.JSON(&w, i.A{0})
	} else {
		i.JSON(&w, i.A{2})
	}
}
