package advertisement

import (
	"iter"
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	challengeShared "github.com/luskaner/ageServer/server/internal/routes/game/challenge/shared"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type updateStateRequest struct {
	AdvertisementId int32 `schema:"advertisementid"`
	State           int8  `schema:"state"`
}

func UpdateState(w http.ResponseWriter, r *http.Request) {
	var q updateStateRequest
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	game := models.G(r)
	gameTitle := game.Title()
	advertisements := game.Advertisements()
	battleServers := game.BattleServers()
	sess := models.SessionOrPanic(r)
	var ok bool
	var peers iter.Seq2[int32, models.Peer]
	var advStartTime int64
	var advEncoded i.A
	advertisements.WithWriteLock(q.AdvertisementId, func() {
		var adv models.Advertisement
		adv, ok = game.Advertisements().GetAdvertisement(q.AdvertisementId)
		if !ok {
			i.JSON(&w, i.A{2})
			return
		}
		adv.UnsafeUpdateState(q.State)
		if adv.UnsafeGetState() == 1 {
			_, peers = adv.GetPeers().Iter()
			advEncoded = adv.UnsafeEncode(gameTitle, sess.GetClientLibVersion(), battleServers)
			advStartTime = adv.UnsafeGetStartTime()
		}
		ok = true
	})
	if ok {
		var userIds []i.A
		var userIdStr []i.A
		var races []i.A
		var unsafeChallengeProgress []i.A
		var safeChallengeProgress []i.A
		var sessions []models.Session
		var safeSessions []bool
		gameSessions := game.Sessions()
		for userId, peer := range peers {
			user, userFound := game.Users().GetUserById(userId)
			if !userFound {
				continue
			}
			sess, sessFound := gameSessions.GetByUserId(userId)
			if !sessFound {
				continue
			}
			userIdSingleStr := strconv.Itoa(int(userId))
			userIds = append(userIds, i.A{userId, i.A{}})
			userIdStr = append(userIdStr, i.A{userIdSingleStr, i.A{}})
			peerMutable := peer.GetMutable()
			races = append(races, i.A{userIdSingleStr, strconv.Itoa(int(peerMutable.RaceID))})
			unsafeChallengeProgress = append(unsafeChallengeProgress, i.A{userIdSingleStr, challengeShared.GetChallengeProgressData(user, false)})
			safeChallengeProgress = append(safeChallengeProgress, i.A{userIdSingleStr, challengeShared.GetChallengeProgressData(user, true)})
			safeSessions = append(safeSessions, i.SafeEnabled(sess.Official(), user.GetPlatformUserID()))
			sessions = append(sessions, sess)
		}
		for index, session := range sessions {
			progress := unsafeChallengeProgress
			if safeSessions[index] {
				progress = safeChallengeProgress
			}
			wss.SendOrStoreMessage(
				session,
				"MatchStartMessage",
				i.A{
					userIds,
					races,
					advStartTime,
					userIdStr,
					advEncoded,
					progress,
				},
			)
		}
		i.JSON(&w, i.A{0})
	} else {
		i.JSON(&w, i.A{2})
	}
}
