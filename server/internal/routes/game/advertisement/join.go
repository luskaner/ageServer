package advertisement

import (
	"fmt"
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

type JoinRequest struct {
	shared.AdvertisementBaseRequest
	Password string `schema:"password"`
}

func joinReturnError(battleServers models.BattleServers, r *http.Request, w http.ResponseWriter) {
	battleServer := battleServers.NewBattleServer("")
	response := encodeJoinResponse(2, "", battleServer, r, i.A{})
	i.JSON(&w, response)
}

func encodeJoinResponse(errorCode int, ip string, battleServer models.BattleServer, r *http.Request, peerEncoded i.A) i.A {
	response := i.A{
		errorCode,                   // resultCode
		ip,                          // host IP
		battleServer.ResolveIPv4(r), // resolved IP
	}
	response = append(response, battleServer.EncodePorts()...) // ports
	response = append(response, i.A{peerEncoded})              // peer
	return response
}

func Join(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	battleServers := g.BattleServers()
	var q JoinRequest
	if err := i.Bind(r, &q); err != nil {
		joinReturnError(battleServers, r, w)
		return
	}
	sess := models.SessionOrPanic(r)

	u, ok := g.Users().GetUserById(sess.GetUserId())
	if !ok {
		joinReturnError(battleServers, r, w)
		return
	}
	advertisements := g.Advertisements()
	// Leave the previous match if the user is already in one
	// Necessary for AoE1 but might as well do it for all (except AoE4 which needs multiple for groups)
	if g.Title() != game.AoE4 {
		// FIXME: Exit in aoe4 if the currrent match is not a party
		if existingAdv := advertisements.GetUserAdvertisement(u.GetId()); existingAdv != nil {
			advertisements.WithWriteLock(existingAdv.GetId(), func() {
				advertisements.UnsafeRemovePeer(existingAdv.GetId(), u.GetId())
			})
		}
	}
	matchingAdv, foundAdv := advertisements.GetAdvertisement(q.Id)
	if !foundAdv {
		joinReturnError(battleServers, r, w)
		return
	}
	var automatch bool
	advertisements.WithReadLock(matchingAdv.GetId(), func() {
		automatch = matchingAdv.UnsafeIsAutomatch()
	})
	if automatch {
		logger.Printf(
			"automatch2: join request user=%d adv=%d party=%d race=%d team=%d\n",
			u.GetId(),
			q.Id,
			q.Party,
			q.Race,
			q.Team,
		)
	}
	if !automatch && q.Party != -1 && q.Party != matchingAdv.GetParty() {
		joinReturnError(battleServers, r, w)
		return
	}
	if automatch {
		// The matchmaking join request can carry the player's party session
		// and synthetic race/team values. Teams are assigned by the matched
		// polling response; the advertisement itself is a server-managed
		// solo lobby.
		q.Party = -1
		q.Race = -1
		q.Team = -1
	}
	var rejectReason string
	advertisements.WithReadLock(matchingAdv.GetId(), func() {
		switch {
		case !automatch && !matchingAdv.UnsafeGetJoinable():
			rejectReason = "advertisement is not joinable"
		case matchingAdv.UnsafeGetAppBinaryChecksum() != q.AppBinaryChecksum:
			rejectReason = fmt.Sprintf(
				"app binary checksum mismatch (host=%d join=%d)",
				matchingAdv.UnsafeGetAppBinaryChecksum(),
				q.AppBinaryChecksum,
			)
		case matchingAdv.UnsafeGetDataChecksum() != q.DataChecksum:
			rejectReason = fmt.Sprintf(
				"data checksum mismatch (host=%d join=%d)",
				matchingAdv.UnsafeGetDataChecksum(),
				q.DataChecksum,
			)
		case matchingAdv.UnsafeGetModDllFile() != q.ModDllFile ||
			matchingAdv.UnsafeGetModDllChecksum() != q.ModDllChecksum:
			rejectReason = fmt.Sprintf(
				"mod dll mismatch (host=%q/%d join=%q/%d)",
				matchingAdv.UnsafeGetModDllFile(),
				matchingAdv.UnsafeGetModDllChecksum(),
				q.ModDllFile,
				q.ModDllChecksum,
			)
		case matchingAdv.UnsafeGetModName() != q.ModName ||
			matchingAdv.UnsafeGetModVersion() != q.ModVersion:
			rejectReason = fmt.Sprintf(
				"mod version mismatch (host=%q/%q join=%q/%q)",
				matchingAdv.UnsafeGetModName(),
				matchingAdv.UnsafeGetModVersion(),
				q.ModName,
				q.ModVersion,
			)
		case matchingAdv.UnsafeGetVersionFlags() != q.VersionFlags:
			rejectReason = fmt.Sprintf(
				"version flags mismatch (host=%d join=%d)",
				matchingAdv.UnsafeGetVersionFlags(),
				q.VersionFlags,
			)
		case !automatch && matchingAdv.UnsafeGetPasswordValue() != q.Password:
			rejectReason = "password mismatch"
		}
	})
	if rejectReason != "" {
		if automatch {
			logger.Printf(
				"automatch2: join rejected user=%d adv=%d reason=%s\n",
				u.GetId(),
				q.Id,
				rejectReason,
			)
		}
		joinReturnError(battleServers, r, w)
		return
	}
	if automatch {
		logger.Printf(
			"automatch2: join accepted user=%d adv=%d party=%d race=%d team=%d\n",
			u.GetId(),
			q.Id,
			q.Party,
			q.Race,
			q.Team,
		)
	}
	var response i.A
	advertisements.WithWriteLock(matchingAdv.GetId(), func() {
		peer := advertisements.UnsafeNewPeer(
			matchingAdv.GetId(),
			matchingAdv.GetIp(),
			u.GetId(),
			u.GetStatGroupID(),
			q.Party,
			q.Race,
			q.Team,
		)
		if peer == nil {
			ok = false
			if automatch {
				logger.Printf("automatch2: join failed user=%d adv=%d (could not add peer)\n", u.GetId(), q.Id)
			}
			return
		}
		battleServer, battleServerExists := battleServers.Get(matchingAdv.GetRelayRegion())
		if !battleServerExists {
			battleServer = battleServers.NewLANBattleServer("")
		}
		response = encodeJoinResponse(
			0,
			matchingAdv.GetIp(),
			battleServer,
			r,
			peer.Encode(),
		)
	})
	if ok {
		i.JSON(&w, response)
	} else {
		joinReturnError(battleServers, r, w)
	}
}
