package advertisement

import (
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

func updateReturnError(gameId string, w *http.ResponseWriter) {
	response := i.A{2}
	if gameId != game.AoE1 {
		response = append(response, i.A{})
	}
	i.JSON(w, response)
}

func Update(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	gameTitle := g.Title()

	var q shared.AdvertisementUpdateRequest
	if err := i.Bind(r, &q); err != nil {
		updateReturnError(gameTitle, &w)
		return
	}
	advertisements := g.Advertisements()
	battleServers := g.BattleServers()
	sess := models.SessionOrPanic(r)
	var response i.A
	var ok bool
	var automatch bool
	var createdAutomatch bool
	notifications := []models.MatchNotification(nil)
	advertisements.WithWriteLock(q.Id, func() {
		var adv models.Advertisement
		adv, ok = advertisements.GetAdvertisement(q.Id)
		if !ok {
			if gameTitle != game.AoE2 {
				return
			}
			// The AoE2:DE matchmaking host never calls /advertisement/host.
			// After the matched polling response it updates the reserved
			// advertisement ID directly. Create that server-managed
			// advertisement from the update request.
			hostID := q.HostId
			if hostID == 0 {
				hostID = sess.GetUserId()
			}
			if hostID != sess.GetUserId() {
				return
			}
			u, found := g.Users().GetUserById(hostID)
			if !found {
				return
			}
			reservation, reserved := g.Matchmaking().PrepareHostAdvertisement(hostID, q.Id)
			if !reserved {
				return
			}
			hostRequest := &shared.AdvertisementHostRequest{
				AdvertisementBaseRequest: shared.AdvertisementBaseRequest{
					Id:                reservation.MatchID,
					AppBinaryChecksum: q.AppBinaryChecksum,
					DataChecksum:      q.DataChecksum,
					ModDllFile:        q.ModDllFile,
					ModDllChecksum:    q.ModDllChecksum,
					ModName:           q.ModName,
					ModVersion:        q.ModVersion,
					Party:             -1,
					VersionFlags:      q.VersionFlags,
				},
				Description:      models.AutomatchMatchDescription,
				AutomatchPollId:  q.AutomatchPollId,
				RelayRegion:      reservation.BattleServer.Region(),
				MapName:          reservation.MapName,
				HostId:           hostID,
				Observable:       q.Observable,
				ObserverDelay:    q.ObserverDelay,
				ObserverPassword: q.ObserverPassword,
				Password:         q.Password,
				Passworded:       q.Passworded,
				ModDllFile:       q.ModDllFile,
				ModDllChecksum:   q.ModDllChecksum,
				Visible:          q.Visible,
				Joinable:         true,
				MatchType:        models.AutomatchAdvertisementMatchType,
				MaxPlayers:       reservation.MaxPlayers,
				Options:          q.Options,
				SlotInfo:         q.SlotInfo,
				PsnSessionId:     q.PsnSessionId,
				State:            q.State,
				ServiceType:      0,
			}
			adv, ok = advertisements.StoreWithId(hostRequest, reservation.MatchID, false, gameTitle)
			if !ok || adv == nil {
				return
			}
			adv.UnsafeSetAutomatch(true)
			if reservation.PlatformSessionID != 0 {
				// The host announced the platform session before creating the
				// advertisement, so the advertisement did not exist yet. Apply
				// the stored value now; the joiner reads it from the ad.
				adv.UnsafeUpdatePlatformSessionId(reservation.PlatformSessionID)
			}
			if advertisements.UnsafeNewPeer(adv.GetId(), adv.GetIp(), u.GetId(), u.GetStatGroupID(), -1, 0, 0) == nil {
				advertisements.UnsafeDelete(adv)
				ok = false
				return
			}
			automatch = true
			createdAutomatch = true
			logger.Printf(
				"automatch2: host advertisement created from update adv=%d host=%d map=%q max=%d matchtype=%d relay=%q\n",
				adv.GetId(),
				hostID,
				reservation.MapName,
				reservation.MaxPlayers,
				models.AutomatchAdvertisementMatchType,
				reservation.BattleServer.Region(),
			)
		}

		if gameTitle == game.AoE1 || i.BeforeTheBalticPowers(gameTitle, sess.GetClientLibVersion()) {
			q.Joinable = true
		}
		if !createdAutomatch {
			automatch = adv.UnsafeIsAutomatch()
		}
		if automatch {
			adv.UnsafeUpdateMatchmaking(&q)
		} else {
			adv.UnsafeUpdate(&q)
		}
		if gameTitle == game.AoE1 || i.BeforeTheBalticPowers(gameTitle, sess.GetClientLibVersion()) {
			adv.UnsafeUpdatePlatformSessionId(q.PsnSessionId)
		}

		if gameTitle == game.AoE2 || i.SinceTheBalticPowers(gameTitle, sess.GetClientLibVersion()) || gameTitle == game.AoM || gameTitle == game.AoE4 {
			response = adv.UnsafeEncode(gameTitle, sess.GetClientLibVersion(), battleServers)
		}
		ok = true
	})
	if createdAutomatch {
		notifications = g.Matchmaking().HostCreated(0, q.Id, sess.GetUserId())
	}
	if !ok {
		updateReturnError(gameTitle, &w)
		return
	}
	if automatch {
		logger.Printf(
			"automatch2: host advertisement update adv=%d user=%d desc=%q map=%q max=%d matchtype=%d state=%d joinable=%t\n",
			q.Id,
			sess.GetUserId(),
			q.Description,
			q.MapName,
			q.MaxPlayers,
			q.MatchType,
			q.State,
			q.Joinable,
		)
	}
	if response == nil {
		i.JSON(&w,
			i.A{
				0,
			},
		)
	} else {
		i.JSON(&w,
			i.A{
				0,
				response,
			},
		)
	}
	wss.SendMatchNotifications(notifications)
}
