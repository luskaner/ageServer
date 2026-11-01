package advertisement

import (
	"iter"
	"net/http"
	"strconv"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type request struct {
	MatchID int32 `schema:"matchID"`
}

func updatePlatformID(w *http.ResponseWriter, r *http.Request, idKey string) {
	var req request
	if err := i.Bind(r, &req); err != nil {
		i.JSON(w, i.A{2})
		return
	}

	g := models.G(r)
	advertisements := g.Advertisements()
	var currentUserId int32
	var peersId iter.Seq[int32]
	var ok bool
	var metadata string
	idValue, err := strconv.ParseInt(r.PostFormValue(idKey), 10, 64)
	if err != nil {
		i.JSON(w, i.A{2})
		return
	}
	idValueUint := uint64(idValue)
	sess := models.SessionOrPanic(r)
	advertisements.WithWriteLock(req.MatchID, func() {
		var adv models.Advertisement
		adv, ok = advertisements.GetAdvertisement(req.MatchID)
		if !ok {
			return
		}

		currentUserId = sess.GetUserId()
		peers := adv.GetPeers()
		if _, ok = peers.Load(currentUserId); !ok {
			return
		}

		adv.UnsafeUpdatePlatformSessionId(idValueUint)
		metadata = adv.GetXboxSessionId()
		_, peersId = peers.Keys()
		ok = true
	})
	if !ok {
		// The AoE2:DE matchmaking host announces the platform session for the
		// reserved match ID before it creates the advertisement. Keep it on the
		// pending match so the advertisement can expose it later.
		if g.Matchmaking().SetPlatformSessionID(sess.GetUserId(), req.MatchID, idValueUint) {
			i.JSON(w, i.A{0})
			return
		}
		i.JSON(w, i.A{2})
		return
	}
	sessions := g.Sessions()
	message := i.A{req.MatchID, metadata, idValueUint}
	if gameTitle := g.Title(); i.SinceTheBalticPowers(gameTitle, sess.GetClientLibVersion()) || gameTitle == game.AoE2 || gameTitle == game.AoE4 || gameTitle == game.AoM {
		message = append(message, 0, "", "")
	}
	for peerId := range peersId {
		if currentSess, ok := sessions.GetByUserId(peerId); ok {
			wss.SendOrStoreMessage(
				currentSess,
				"PlatformSessionUpdateMessage",
				message,
			)
		}
	}

	i.JSON(w,
		i.A{0},
	)
}

func UpdatePlatformLobbyID(w http.ResponseWriter, r *http.Request) {
	updatePlatformID(&w, r, "platformlobbyID")
}
