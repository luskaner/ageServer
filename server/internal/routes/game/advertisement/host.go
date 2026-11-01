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

func returnError(battleServers models.BattleServers, gameId string, clientLibVersion uint16, r *http.Request, w *http.ResponseWriter) {
	battleServer := battleServers.NewBattleServer("")
	response := encodeHostResponse(
		gameId,
		clientLibVersion,
		2,
		0,
		battleServer,
		r,
		"",
		[]i.A{},
		"0",
		"",
	)
	i.JSON(w, response)
}

func encodeHostResponse(gameTitle string, clientLibVersion uint16, errorCode int, advId int32, battleServer models.BattleServer, r *http.Request, relayRegion string, encodedPeers []i.A, metadata string, description string) i.A {
	response := i.A{
		errorCode,
		advId,
		"authtoken",
	}
	response = append(response, battleServer.EncodeAdvertisement(r)...)
	response = append(
		response,
		relayRegion,
		encodedPeers,
		0,
	)
	switch {
	case gameTitle == game.AoE1:
		response = append(response, metadata)
	case gameTitle == game.AoE2 || i.SinceTheBalticPowers(gameTitle, clientLibVersion) || gameTitle == game.AoE4 || gameTitle == game.AoM:
		response = append(
			response,
			0,
		)
		if gameTitle == game.AoE3 {
			response = append(response, "")
		} else {
			response = append(response, nil)
		}
		response = append(response, nil, metadata, description)
	default:
		response = append(response, "0")
	}
	return response
}

func Host(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	gameTitle := g.Title()
	battleServers := g.BattleServers()
	sess := models.SessionOrPanic(r)

	var adv shared.AdvertisementHostRequest
	if err := i.Bind(r, &adv); err != nil {
		returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
		return
	}

	if gameTitle != game.AoE2 && gameTitle != game.AoE4 && models.IsAutomatchDescription(adv.Description) {
		// Matchmaking is only implemented for AoE2:DE. Keep the previous
		// behaviour for the other titles.
		returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
		return
	}
	// SESSION_MATCH_KEY is used both by ordinary session/party lobbies
	// (matchtype 1, four empty slots) and by the actual matchmaking host
	// (matchtype 19, no_map). Only the latter is server-managed.
	automatch := gameTitle == game.AoE2 &&
		(adv.AutomatchPollId > 0 || adv.MatchType == models.AutomatchAdvertisementMatchType)
	var reservation models.AutomatchHostReservation
	var battleServer models.BattleServer

	if automatch {
		// Keep the server-side marker even if the client omitted the
		// description. The advertisement flag below is what the join/update
		// routes use to recognize the server-managed matchmaking lobby.
		adv.Description = models.AutomatchMatchDescription
		logger.Printf(
			"automatch2: host request user=%d adv=%d poll=%d desc=%q matchtype=%d state=%d map=%q max=%d party=%d relay=%q\n",
			adv.HostId,
			adv.Id,
			adv.AutomatchPollId,
			adv.Description,
			adv.MatchType,
			adv.State,
			adv.MapName,
			adv.MaxPlayers,
			adv.Party,
			adv.RelayRegion,
		)
		if adv.HostId == 0 {
			adv.HostId = sess.GetUserId()
		}
		if adv.HostId != sess.GetUserId() {
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		}
		// Matchmaking is a server-managed solo lobby. Ignore a party value
		// supplied by the client (solo clients may send 0 instead of -1).
		adv.Party = -1
		var ok bool
		reservation, ok = g.Matchmaking().PrepareHost(adv.HostId, adv.AutomatchPollId)
		if !ok {
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		}
		if adv.Id != -1 && adv.Id != reservation.MatchID {
			g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		}
		if reservation.MaxPlayers != 0 {
			adv.MaxPlayers = reservation.MaxPlayers
		}
		adv.MapName = reservation.MapName
		adv.MatchType = models.AutomatchAdvertisementMatchType
		adv.RelayRegion = reservation.BattleServer.Region()
		adv.Joinable = true
		// The matchmaking host request carries synthetic race/team values.
		// The server assigns teams in the matched polling response instead.
		adv.Race = 0
		battleServer = reservation.BattleServer
		logger.Printf(
			"automatch2: host reservation match=%d user=%d map=%s max=%d matchtype=%d relay=%q\n",
			reservation.MatchID,
			adv.HostId,
			adv.MapName,
			adv.MaxPlayers,
			adv.MatchType,
			adv.RelayRegion,
		)
	} else {
		if gameTitle == game.AoE2 && models.IsAutomatchDescription(adv.Description) {
			logger.Printf(
				"automatch2: session host user=%d matchtype=%d map=%q max=%d party=%d relay=%q\n",
				adv.HostId,
				adv.MatchType,
				adv.MapName,
				adv.MaxPlayers,
				adv.Party,
				adv.RelayRegion,
			)
		}
		region := r.PostFormValue("relayRegion")
		battleServer = battleServers.NewBattleServer(region)
		if !battleServer.LAN() {
			var ok bool
			if battleServer, ok = battleServers.Get(region); !ok {
				returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
				return
			}
		}
		if adv.Id != -1 {
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		}
	}

	if gameTitle == game.AoE1 || i.BeforeTheBalticPowers(gameTitle, sess.GetClientLibVersion()) || gameTitle == game.AoM {
		adv.Joinable = true
	}

	u, ok := g.Users().GetUserById(adv.HostId)
	if !ok {
		if automatch {
			g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
		}
		returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
	}

	advertisements := g.Advertisements()
	// Leave the previous match if the user is already in one.
	// Necessary for AoE3 but might as well do it for all (except AoE4 which
	// needs multiple for groups).
	if gameTitle != game.AoE4 {
		if existingAdv := advertisements.GetUserAdvertisement(u.GetId()); existingAdv != nil &&
			(!automatch || existingAdv.GetId() != reservation.MatchID) {
			advertisements.WithWriteLock(existingAdv.GetId(), func() {
				advertisements.UnsafeRemovePeer(existingAdv.GetId(), u.GetId())
			})
		}
	}
	if adv.Party != -1 {
		if partyAdv, partyOk := advertisements.GetAdvertisement(adv.Party); !partyOk {
			if automatch {
				g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
			}
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		} else if partyAdv.GetParty() != -1 {
			if automatch {
				g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
			}
			returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
			return
		}
	}

	var storedAdv models.Advertisement
	stored := true
	if automatch {
		storedAdv, stored = advertisements.StoreWithId(
			&adv,
			reservation.MatchID,
			// Ranked matchmaking advertisements are server-managed and use
			// the same "0" metadata as the official service.
			false,
			gameTitle,
		)
		if !stored {
			// The request may have been retried after a lost response. Reuse
			// the advertisement that already belongs to this match.
			storedAdv, stored = advertisements.GetAdvertisement(reservation.MatchID)
		}
	} else {
		storedAdv = advertisements.Store(
			&adv,
			!battleServer.LAN(),
			gameTitle,
		)
	}
	if !stored || storedAdv == nil {
		if automatch {
			g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
		}
		returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
		return
	}

	success := true
	var response i.A
	advertisements.WithWriteLock(storedAdv.GetId(), func() {
		storedAdv.UnsafeSetAutomatch(automatch)
		if advertisements.UnsafeNewPeer(storedAdv.GetId(), storedAdv.GetIp(), u.GetId(), u.GetStatGroupID(), adv.Party, adv.Race, adv.Team) == nil {
			success = false
			return
		}
		metadata := storedAdv.GetXboxSessionId()
		if automatch {
			// Ranked matchmaking uses the server-assigned advertisement, not
			// a client-created Xbox session.
			metadata = "0"
		}
		response = encodeHostResponse(
			gameTitle,
			sess.GetClientLibVersion(),
			0,
			storedAdv.GetId(),
			battleServer,
			r,
			storedAdv.GetRelayRegion(),
			storedAdv.EncodePeers(),
			metadata,
			storedAdv.UnsafeGetDescription(),
		)
		if automatch {
			logger.Printf(
				"automatch2: host advertisement created adv=%d host=%d map=%q max=%d matchtype=%d joinable=%t peers=%d\n",
				storedAdv.GetId(),
				adv.HostId,
				adv.MapName,
				adv.MaxPlayers,
				adv.MatchType,
				adv.Joinable,
				storedAdv.GetPeers().Len(),
			)
		}
	})
	if !success {
		if automatch {
			g.Matchmaking().HostFailed(sess.GetUserId(), adv.AutomatchPollId)
		}
		returnError(battleServers, gameTitle, sess.GetClientLibVersion(), r, &w)
		return
	}

	i.JSON(&w, response)
	if automatch {
		wss.SendMatchNotifications(g.Matchmaking().HostCreated(adv.AutomatchPollId, storedAdv.GetId(), u.GetId()))
	}
}
