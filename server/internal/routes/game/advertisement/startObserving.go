package advertisement

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

func joinReturnStartObservingError(battleServers models.BattleServers, r *http.Request, w http.ResponseWriter) {
	battleServer := battleServers.NewBattleServer("")
	response := encodeStartObservingResponse(2, battleServer, r, i.A{}, i.A{}, 0)
	i.JSON(&w, response)
}

func encodeStartObservingResponse(errorCode int, battleServer models.BattleServer, r *http.Request, userIdsInt i.A, userIdsStr i.A, startTime int64) i.A {
	response := i.A{
		errorCode,
		battleServer.ResolveIPv4(r),
	}
	response = append(response, battleServer.EncodePorts()...)
	response = append(response, userIdsInt, startTime, userIdsStr)
	return response
}

func StartObserving(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	battleServers := game.BattleServers()
	var q searchQuery
	if err := i.Bind(r, &q); err != nil {
		joinReturnStartObservingError(battleServers, r, w)
		return
	}
	var a shared.AdvertisementId
	if err := i.Bind(r, &a); err != nil {
		joinReturnStartObservingError(battleServers, r, w)
		return
	}
	sess := models.SessionOrPanic(r)
	currentUserId := sess.GetUserId()
	advertisements := game.Advertisements()

	adv, foundAdv := advertisements.GetAdvertisement(a.AdvertisementId)
	if !foundAdv {
		joinReturnStartObservingError(battleServers, r, w)
		return
	}
	advertisements.WithReadLock(adv.GetId(), func() {
		_, matchesBattleServer := battleServers.Get(adv.GetRelayRegion())
		peers := adv.GetPeers()
		_, isPeer := peers.Load(currentUserId)
		if adv.UnsafeGetJoinable() &&
			isPeer &&
			adv.UnsafeGetVisible() &&
			adv.UnsafeGetAppBinaryChecksum() != q.AppBinaryChecksum &&
			adv.UnsafeGetDataChecksum() != q.DataChecksum &&
			(q.MatchType == nil || adv.UnsafeGetMatchType() != *q.MatchType) &&
			adv.UnsafeGetModDllFile() != q.ModDllFile &&
			adv.UnsafeGetModDllChecksum() != q.ModDllChecksum &&
			adv.UnsafeGetModName() != q.ModName &&
			adv.UnsafeGetModVersion() != q.ModVersion &&
			adv.UnsafeGetVersionFlags() != q.VersionFlags &&
			!adv.UnsafeGetObserversEnabled() &&
			!matchesBattleServer {
			joinReturnStartObservingError(battleServers, r, w)
			return
		}
	})
	var response i.A
	advertisements.WithReadLock(adv.GetId(), func() {
		battleServer, battleServerExists := game.BattleServers().Get(adv.GetRelayRegion())
		if !battleServerExists {
			joinReturnStartObservingError(battleServers, r, w)
			return
		}
		peers := adv.GetPeers()
		peerLength, userIds := peers.Keys()
		userIdsInt := make(i.A, peerLength)
		userIdsStr := make(i.A, peerLength)
		j := 0
		for userId := range userIds {
			userIdsInt[j] = i.A{userId, i.A{}}
			userIdsStr[j] = i.A{strconv.Itoa(int(userId)), i.A{}}
			j++
		}
		response = encodeStartObservingResponse(
			0,
			battleServer,
			r,
			userIdsInt,
			userIdsStr,
			adv.UnsafeGetStartTime(),
		)
	})
	adv.StartObserving(sess.GetUserId())
	i.JSON(&w, response)
}
