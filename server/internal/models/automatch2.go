package models

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

const (
	// AutomatchMatchDescription is the description used by the AoE2:DE
	// matchmaking client when it asks the API to create the host
	// advertisement.
	AutomatchMatchDescription = "SESSION_MATCH_KEY"
	automatchGameMessage      = "Automatch2GameMessage"
	automatchHostTimeout      = 45 * time.Second
	// AutomatchAdvertisementMatchType is the advertisement matchtype used by
	// the official AoE2:DE matchmaking host request.
	AutomatchAdvertisementMatchType uint8 = 19
	// The AoE2:DE client always expects the canonical 1v1 Random Map queue
	// in the response, regardless of the matchTypes sent while polling.
	automatchQueueID            int32 = 6
	automatchQueueLeaderboardID int32 = 6
	automatchQueueMaxPlayers          = 2
	automatchQueueTeamSize            = 1
)

// LeaderboardForQueue maps an automatch queue to the ladder that the client
// queries for it. Queue id 6 is the 1v1 random map automatch queue, whose
// ladder is SOLO_RM_RANKED (3).
func LeaderboardForQueue(queueID int32) int32 {
	if queueID == automatchQueueID {
		return SoloRMRanked
	}
	return 0
}

// AutomatchPollingRequest is the form body sent by the client to
// /game/automatch2/polling.
type AutomatchPollingRequest struct {
	AppBinCRC         int32            `schema:"appBinCRC"`
	CallNum           int32            `schema:"callNum"`
	ConnectID         string           `schema:"connect_id"`
	DataCRC           int32            `schema:"dataCRC"`
	FactionIDs        i.Json[[]int32]  `schema:"factionIDs"`
	LastCallTime      int64            `schema:"lastCallTime"`
	MatchTypes        i.Json[[]int32]  `schema:"matchTypes"`
	ModDllFile        string           `schema:"modDLLFile"`
	ModDllCRC         int32            `schema:"modDllCRC"`
	ModName           string           `schema:"modName"`
	ModVersion        string           `schema:"modVersion"`
	Options           i.Json[[]string] `schema:"options"`
	PartySessionID    int64            `schema:"partySessionID"`
	RaceInfoKey       i.Json[[]int32]  `schema:"raceInfoKey"`
	RaceInfoProfileID i.Json[[]int32]  `schema:"raceInfoProfileID"`
	RaceInfoRaceID    i.Json[[]int32]  `schema:"raceInfoRaceID"`
	RelayPingTimes    i.Json[[]int32]  `schema:"relayPingTimes"`
	RelayRegion       string           `schema:"relayRegion"`
	RelayRegions      i.Json[[]string] `schema:"relayRegions"`
	SessionID         string           `schema:"sessionID"`
	VersionFlags      uint32           `schema:"versionFlags"`
	VetoMapKey        i.Json[[]int32]  `schema:"vetoMapKey"`
	VetoMaps          i.Json[[]string] `schema:"vetoMaps"`
}

type AutomatchStopPollingRequest struct {
	CallNum        int32  `schema:"callNum"`
	Commit         int32  `schema:"commit"`
	ConnectID      string `schema:"connect_id"`
	LastCallTime   int64  `schema:"lastCallTime"`
	OwnerProfileID int32  `schema:"ownerProfileID"`
	SessionID      string `schema:"sessionID"`
}

type AutomatchUpdateStatusRequest struct {
	CallNum      int32  `schema:"callNum"`
	ConnectID    string `schema:"connect_id"`
	LastCallTime int64  `schema:"lastCallTime"`
	MatchID      int32  `schema:"matchID"`
	Result       int32  `schema:"result"`
	ResultCode   int32  `schema:"resultCode"`
	SessionID    string `schema:"sessionID"`
}

// AutomatchHostReservation is everything the advertisement route needs to
// create the host side of a match that has already been paired.
type AutomatchHostReservation struct {
	MatchID           int32
	BattleServer      BattleServer
	MapName           string
	MaxPlayers        uint8
	PlatformSessionID uint64
}

type MatchNotification struct {
	Session Session
	Action  string
	Message any
}

type automatchQueue struct {
	ID            int32
	MaxPlayers    int
	TeamSize      int
	LeaderboardID int32
}

type automatchQueueEntry struct {
	Session        Session
	UserID         int32
	PollID         int32
	PartySessionID int64
	MatchTypes     []int32
	RelayRegion    string
	MatchID        int32
	Stopped        bool
	LocalIP        string
	CreatedAt      time.Time
	VetoMapKeys    []int32
	VetoMapNames   []string
}

type automatchMatchState uint8

const (
	automatchWaitingHost automatchMatchState = iota
	automatchHosting
	automatchReady
)

type automatchMatch struct {
	ID                int32
	HostID            int32
	HostPollID        int32
	Players           []*automatchQueueEntry
	Queue             automatchQueue
	MapName           string
	MapID             int32
	RelayRegion       string
	State             automatchMatchState
	AdvertisementID   int32
	PlatformSessionID uint64
	JoinNotified      bool
	CreatedAt         time.Time
}

// Matchmaking owns the automatch2 queue. It returns notifications instead of
// importing the wss package, since wss imports this package.
type Matchmaking interface {
	Initialize(game Game)
	Poll(session Session, request *AutomatchPollingRequest, r *http.Request) (i.A, []MatchNotification)
	StopPolling(session Session, request *AutomatchStopPollingRequest) i.A
	UpdateStatus(session Session, request *AutomatchUpdateStatusRequest) i.A
	PrepareHost(hostID int32, pollID int32) (AutomatchHostReservation, bool)
	PrepareHostAdvertisement(hostID int32, advertisementID int32) (AutomatchHostReservation, bool)
	HostCreated(pollID int32, advertisementID int32, hostID int32) []MatchNotification
	SetPlatformSessionID(userID int32, matchID int32, platformSessionID uint64) bool
	HostFailed(hostID int32, pollID int32)
	RemoveUser(userID int32)
}

type MainMatchmaking struct {
	mu      sync.Mutex
	game    Game
	queue   map[int32]*automatchQueueEntry
	byUser  map[int32]*automatchQueueEntry
	byPoll  map[int32]*automatchQueueEntry
	matches map[int32]*automatchMatch
}

func (mm *MainMatchmaking) Initialize(game Game) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.game = game
	mm.queue = make(map[int32]*automatchQueueEntry)
	mm.byUser = make(map[int32]*automatchQueueEntry)
	mm.byPoll = make(map[int32]*automatchQueueEntry)
	mm.matches = make(map[int32]*automatchMatch)
}

func (mm *MainMatchmaking) Poll(session Session, request *AutomatchPollingRequest, r *http.Request) (i.A, []MatchNotification) {
	if mm.game == nil || session == nil {
		return AutomatchUnmatchedResponse(0, nil), nil
	}
	if request == nil {
		request = &AutomatchPollingRequest{}
	}

	userID := session.GetUserId()
	matchTypes := cleanMatchTypes(request.MatchTypes.Data)

	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, exists := mm.byUser[userID]
	if !exists {
		entry = &automatchQueueEntry{
			UserID:    userID,
			PollID:    mm.newPollIDLocked(),
			CreatedAt: time.Now(),
		}
		mm.byUser[userID] = entry
		mm.byPoll[entry.PollID] = entry
	}
	previousMatchTypes := entry.MatchTypes
	previousPartySessionID := entry.PartySessionID
	previousRelayRegion := entry.RelayRegion
	wasStopped := entry.Stopped
	if wasStopped && entry.MatchID == 0 {
		entry.CreatedAt = time.Now()
	}
	entry.Stopped = false
	entry.Session = session
	if r != nil {
		if ip := localIp(r); ip != "" {
			entry.LocalIP = ip
		}
	}
	entry.PartySessionID = request.PartySessionID
	entry.RelayRegion = request.RelayRegion
	entry.VetoMapKeys = append([]int32(nil), request.VetoMapKey.Data...)
	entry.VetoMapNames = append([]string(nil), request.VetoMaps.Data...)
	if !exists || wasStopped || !slices.Equal(previousMatchTypes, matchTypes) ||
		previousPartySessionID != request.PartySessionID || previousRelayRegion != request.RelayRegion {
		logger.Printf(
			"automatch2: poll user=%d poll=%d call=%d matchTypes=%v party=%d relay=%q profiles=%v races=%v factions=%v vetoMapKey=%v vetoMaps=%v vetoes=%d\n",
			userID,
			entry.PollID,
			request.CallNum,
			matchTypes,
			request.PartySessionID,
			request.RelayRegion,
			request.RaceInfoProfileID.Data,
			request.RaceInfoRaceID.Data,
			request.FactionIDs.Data,
			request.VetoMapKey.Data,
			request.VetoMaps.Data,
			len(request.VetoMaps.Data),
		)
	}

	if entry.MatchID != 0 {
		if match, ok := mm.matches[entry.MatchID]; ok {
			switch match.State {
			case automatchReady:
				if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
					return response, nil
				}
				// The advertisement disappeared. Give both players another
				// chance instead of leaving them waiting forever.
				match.State = automatchHosting
				mm.cancelMatchLocked(match, true, 0)
				entry.MatchID = 0
			case automatchWaitingHost, automatchHosting:
				// The HTTP polling response is what tells the host to create
				// the advertisement in AoE2:DE. The advertisement does not
				// exist yet, so answer with the reserved match ID and the
				// selected BattleServer. The joiner keeps polling until the
				// host has created the advertisement.
				if entry.UserID == match.HostID {
					if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
						return response, nil
					}
				}
				return automatchUnmatchedResponse(entry.PollID, matchTypes, automatchWaitSeconds(entry)), nil
			}
		} else {
			entry.MatchID = 0
		}
	}

	if len(matchTypes) == 0 {
		mm.removeFromQueueLocked(entry)
		return automatchUnmatchedResponse(entry.PollID, nil, 0), nil
	}

	entry.MatchTypes = matchTypes
	entry.Session = session
	mm.queue[userID] = entry

	var other *automatchQueueEntry
	for _, candidate := range mm.queue {
		if candidate.UserID == entry.UserID || sameParty(entry, candidate) {
			continue
		}
		if _, ok := compatibleAutomatchQueue(entry.MatchTypes, candidate.MatchTypes); !ok {
			continue
		}
		if other == nil || candidate.CreatedAt.Before(other.CreatedAt) {
			other = candidate
		}
	}
	if other == nil {
		return automatchUnmatchedResponse(entry.PollID, matchTypes, automatchWaitSeconds(entry)), nil
	}

	queue, _ := compatibleAutomatchQueue(entry.MatchTypes, other.MatchTypes)
	if other.CreatedAt.Before(entry.CreatedAt) {
		mm.createMatchLocked(other, entry, queue)
	} else {
		mm.createMatchLocked(entry, other, queue)
	}
	// If the player that just polled is the host, return the matched response
	// immediately. This is the signal that makes the client create the host
	// advertisement. Otherwise the other player is the host and will get the
	// matched response on its next poll.
	if match, ok := mm.matches[entry.MatchID]; ok && entry.UserID == match.HostID {
		if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
			return response, nil
		}
	}
	return automatchUnmatchedResponse(entry.PollID, matchTypes, automatchWaitSeconds(entry)), nil
}

func (mm *MainMatchmaking) StopPolling(session Session, request *AutomatchStopPollingRequest) i.A {
	if mm.game == nil || session == nil {
		return i.A{0}
	}
	userID := session.GetUserId()
	commit := int32(0)
	if request != nil {
		commit = request.Commit
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.byUser[userID]
	if !ok {
		return i.A{0}
	}
	entry.Stopped = true
	mm.removeFromQueueLocked(entry)
	if entry.MatchID != 0 {
		// StopPolling is part of the client's match handshake. Keep the pair
		// alive so the host can create the advertisement and the joiner can
		// receive the match notification. A failed/abandoned host is cleaned
		// up by expireMatch/HostFailed/RemoveUser.
		state := "missing"
		if match, found := mm.matches[entry.MatchID]; found {
			state = strconv.Itoa(int(match.State))
		}
		logger.Printf(
			"automatch2: stop polling user=%d poll=%d commit=%d match=%d state=%s (keeping match)\n",
			userID,
			entry.PollID,
			commit,
			entry.MatchID,
			state,
		)
		return i.A{0}
	}
	mm.removeQueueEntryLocked(entry)
	logger.Printf(
		"automatch2: stop polling user=%d poll=%d commit=%d (left queue)\n",
		userID,
		entry.PollID,
		commit,
	)
	return i.A{0}
}

func (mm *MainMatchmaking) UpdateStatus(session Session, request *AutomatchUpdateStatusRequest) i.A {
	if mm.game == nil || session == nil {
		return i.A{0, 0}
	}
	if request == nil {
		request = &AutomatchUpdateStatusRequest{}
	}
	logger.Printf(
		"automatch2: update status user=%d match=%d result=%d resultCode=%d call=%d\n",
		session.GetUserId(),
		request.MatchID,
		request.Result,
		request.ResultCode,
		request.CallNum,
	)

	// The client reports match progress here. The actual game is owned by the
	// BattleServer, so the API only acknowledges the update. Keeping the
	// advertisement and the match alive is important: the host can report its
	// status before the joiner has received the matched polling response.
	return i.A{0, 0}
}

func (mm *MainMatchmaking) PrepareHost(hostID int32, pollID int32) (AutomatchHostReservation, bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.entryForHostLocked(hostID, pollID)
	if !ok {
		logger.Printf("automatch2: prepare host failed user=%d poll=%d (no pending match)\n", hostID, pollID)
		return AutomatchHostReservation{}, false
	}
	match, ok := mm.matches[entry.MatchID]
	if !ok || match.HostID != entry.UserID {
		logger.Printf(
			"automatch2: prepare host failed user=%d poll=%d match=%d (not host)\n",
			hostID,
			pollID,
			entry.MatchID,
		)
		return AutomatchHostReservation{}, false
	}
	if hostID != 0 && hostID != match.HostID {
		logger.Printf(
			"automatch2: prepare host failed user=%d poll=%d match=%d (host is %d)\n",
			hostID,
			pollID,
			match.ID,
			match.HostID,
		)
		return AutomatchHostReservation{}, false
	}

	battleServer := mm.selectBattleServerLocked(match.RelayRegion)
	if battleServer == nil {
		logger.Printf("automatch2: no BattleServer available for match %d\n", match.ID)
		return AutomatchHostReservation{}, false
	}

	if match.State == automatchReady && match.AdvertisementID != 0 {
		if _, exists := mm.game.Advertisements().GetAdvertisement(match.AdvertisementID); exists {
			logger.Printf(
				"automatch2: reusing host advertisement user=%d poll=%d match=%d adv=%d server=%q\n",
				hostID,
				entry.PollID,
				match.ID,
				match.AdvertisementID,
				battleServer.Region(),
			)
			return AutomatchHostReservation{
				MatchID:           match.ID,
				BattleServer:      battleServer,
				MapName:           match.MapName,
				MaxPlayers:        uint8(match.Queue.MaxPlayers),
				PlatformSessionID: match.PlatformSessionID,
			}, true
		}
		match.State = automatchHosting
		match.AdvertisementID = 0
	}

	match.State = automatchHosting
	match.RelayRegion = battleServer.Region()
	logger.Printf(
		"automatch2: prepare host user=%d poll=%d match=%d host=%d map=%s server=%q\n",
		hostID,
		entry.PollID,
		match.ID,
		match.HostID,
		match.MapName,
		battleServer.Region(),
	)
	return AutomatchHostReservation{
		MatchID:      match.ID,
		BattleServer: battleServer,
		MapName:      match.MapName,
		MaxPlayers:   uint8(match.Queue.MaxPlayers),
	}, true
}

// PrepareHostAdvertisement resolves a pending match from the advertisement ID
// that the AoE2:DE host client uses in its first /advertisement/update call.
// The automatch host does not call /advertisement/host: it assumes the server
// has already allocated the advertisement and only updates its fields.
func (mm *MainMatchmaking) PrepareHostAdvertisement(hostID int32, advertisementID int32) (AutomatchHostReservation, bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.byUser[hostID]
	if !ok || entry.MatchID == 0 || entry.MatchID != advertisementID {
		logger.Printf(
			"automatch2: prepare host advertisement failed user=%d adv=%d (no pending match)\n",
			hostID,
			advertisementID,
		)
		return AutomatchHostReservation{}, false
	}
	match, ok := mm.matches[advertisementID]
	if !ok || match.HostID != hostID {
		logger.Printf(
			"automatch2: prepare host advertisement failed user=%d adv=%d (not host)\n",
			hostID,
			advertisementID,
		)
		return AutomatchHostReservation{}, false
	}

	battleServer := mm.selectBattleServerLocked(match.RelayRegion)
	if battleServer == nil {
		logger.Printf("automatch2: no BattleServer available for match %d\n", match.ID)
		return AutomatchHostReservation{}, false
	}

	match.State = automatchHosting
	match.AdvertisementID = 0
	match.RelayRegion = battleServer.Region()
	logger.Printf(
		"automatch2: prepare host advertisement user=%d match=%d host=%d map=%s server=%q\n",
		hostID,
		match.ID,
		match.HostID,
		match.MapName,
		battleServer.Region(),
	)
	return AutomatchHostReservation{
		MatchID:           match.ID,
		BattleServer:      battleServer,
		MapName:           match.MapName,
		MaxPlayers:        uint8(match.Queue.MaxPlayers),
		PlatformSessionID: match.PlatformSessionID,
	}, true
}

// SetPlatformSessionID remembers the platform session that the AoE2:DE host
// created for a match. The host announces it before it creates the server-side
// advertisement, so it has to be stored on the match and copied into the
// advertisement once the host updates the reserved advertisement ID.
func (mm *MainMatchmaking) SetPlatformSessionID(userID int32, matchID int32, platformSessionID uint64) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	match, ok := mm.matches[matchID]
	if !ok || match.HostID != userID {
		logger.Printf(
			"automatch2: platform session set rejected user=%d match=%d session=%d\n",
			userID,
			matchID,
			platformSessionID,
		)
		return false
	}
	match.PlatformSessionID = platformSessionID
	logger.Printf(
		"automatch2: platform session stored host=%d match=%d session=%d state=%d\n",
		userID,
		matchID,
		platformSessionID,
		match.State,
	)
	return true
}

func (mm *MainMatchmaking) HostCreated(pollID int32, advertisementID int32, hostID int32) []MatchNotification {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.entryForHostLocked(hostID, pollID)
	if !ok {
		logger.Printf("automatch2: host created for unknown host user=%d poll=%d\n", hostID, pollID)
		return nil
	}
	match, ok := mm.matches[entry.MatchID]
	if !ok || match.HostID != hostID || advertisementID != match.ID {
		logger.Printf(
			"automatch2: host created rejected user=%d poll=%d match=%d adv=%d (not host/match)\n",
			hostID,
			pollID,
			entry.MatchID,
			advertisementID,
		)
		return nil
	}
	match.AdvertisementID = advertisementID
	match.State = automatchReady
	logger.Printf(
		"automatch2: host advertisement ready host=%d match=%d adv=%d joiners=%d alreadyNotified=%t\n",
		hostID,
		match.ID,
		advertisementID,
		len(match.Players)-1,
		match.JoinNotified,
	)
	if match.JoinNotified {
		return nil
	}
	match.JoinNotified = true

	if _, ok := mm.game.Advertisements().GetAdvertisement(advertisementID); !ok {
		logger.Printf(
			"automatch2: host advertisement ready but unavailable host=%d match=%d adv=%d\n",
			hostID,
			match.ID,
			advertisementID,
		)
		return nil
	}
	battleServer := mm.selectBattleServerLocked(match.RelayRegion)
	if battleServer == nil {
		logger.Printf("automatch2: no BattleServer for joiner notification match=%d\n", match.ID)
		return nil
	}
	ports := battleServer.EncodePorts()
	for len(ports) < 3 {
		ports = append(ports, 0)
	}

	notifications := make([]MatchNotification, 0, len(match.Players)-1)
	for _, player := range match.Players {
		if player.UserID == match.HostID {
			continue
		}
		// Use the address the joiner used to reach this server instead of the
		// synthetic advertisement IP. The server can have several interfaces
		// (LAN, VPN, ...), and each client must be told the address that is
		// reachable from its own network.
		ip := battleServer.ResolveIPv4ForLocalIP(player.LocalIP)
		if ip == "" {
			logger.Printf(
				"automatch2: no relay address for joiner user=%d localIP=%q match=%d\n",
				player.UserID,
				player.LocalIP,
				match.ID,
			)
		}
		members, hostProfileID := mm.gameMessageMembersLocked(match, ip)
		teamID := int32(1)
		message := map[string]any{
			"hostProfileID":            int64(hostProfileID),
			"matchID":                  int64(match.ID),
			"mapName":                  match.MapName,
			"raceID":                   int64(0),
			"teamID":                   teamID,
			"factionID":                int64(0),
			"teamCount":                int32(match.Queue.TeamSize + 1),
			"maxPlayersPerTeam":        int32(match.Queue.TeamSize),
			"relayAuthToken":           "authtoken",
			"relayIP":                  ip,
			"relayPortRaw":             ports[0],
			"relayPortWebSockets":      ports[1],
			"relayPortOutOfBandSocket": ports[2],
			"matchMembers":             members,
			"options":                  "",
		}
		notifications = append(notifications, MatchNotification{
			Session: player.Session,
			Action:  automatchGameMessage,
			Message: message,
		})
		logger.Printf(
			"automatch2: notifying joiner user=%d poll=%d match=%d host=%d adv=%d map=%s relay=%q peers=%d\n",
			player.UserID,
			player.PollID,
			match.ID,
			match.HostID,
			advertisementID,
			match.MapName,
			ip,
			len(members),
		)
	}
	return notifications
}

func (mm *MainMatchmaking) gameMessageMembersLocked(match *automatchMatch, hostIP string) ([]any, int32) {
	members := make([]any, 0, len(match.Players))
	var hostProfileID int32
	for _, player := range match.Players {
		user, ok := mm.game.Users().GetUserById(player.UserID)
		if !ok {
			continue
		}
		team := int32(1)
		url := ""
		if player.UserID == match.HostID {
			team = 0
			hostProfileID = user.GetId()
			url = hostIP
		}
		members = append(members, map[string]any{
			"sessionID":   int64(user.GetId()),
			"profileID":   int64(user.GetId()),
			"statGroupID": int64(user.GetStatGroupID()),
			"raceID":      int64(0),
			"teamID":      team,
			"ranking":     int32(0),
			"url":         url,
		})
	}
	return members, hostProfileID
}

func (mm *MainMatchmaking) HostFailed(hostID int32, pollID int32) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.entryForHostLocked(hostID, pollID)
	if !ok {
		logger.Printf("automatch2: host failed user=%d poll=%d (no pending match)\n", hostID, pollID)
		return
	}
	match, ok := mm.matches[entry.MatchID]
	if !ok {
		return
	}
	logger.Printf("automatch2: host failed poll=%d match=%d; requeueing players\n", pollID, match.ID)
	mm.cancelMatchLocked(match, true, 0)
}

func (mm *MainMatchmaking) RemoveUser(userID int32) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.byUser[userID]
	if !ok {
		return
	}
	if entry.MatchID != 0 {
		if match, found := mm.matches[entry.MatchID]; found {
			// Only put the other player back in the queue while the host
			// advertisement is still being prepared. Once the match is ready
			// the other player is already joining/playing and must not be
			// silently requeued.
			requeue := match.State == automatchWaitingHost || match.State == automatchHosting
			logger.Printf(
				"automatch2: removing user=%d from match=%d state=%d requeueOther=%t\n",
				userID,
				match.ID,
				match.State,
				requeue,
			)
			mm.cancelMatchLocked(match, requeue, userID)
		}
	}
	mm.removeQueueEntryLocked(entry)
}

func (mm *MainMatchmaking) entryForHostLocked(hostID int32, pollID int32) (*automatchQueueEntry, bool) {
	if pollID > 0 {
		if entry, ok := mm.byPoll[pollID]; ok && entry.MatchID != 0 && (hostID == 0 || entry.UserID == hostID) {
			return entry, true
		}
	}
	if hostID != 0 {
		if entry, ok := mm.byUser[hostID]; ok && entry.MatchID != 0 {
			return entry, true
		}
	}
	return nil, false
}

func (mm *MainMatchmaking) createMatchLocked(host, joiner *automatchQueueEntry, queue automatchQueue) []MatchNotification {
	matchID := mm.newMatchIDLocked()
	mapName, mapID := mm.chooseMap(queue, host, joiner)
	match := &automatchMatch{
		ID:          matchID,
		HostID:      host.UserID,
		HostPollID:  host.PollID,
		Players:     []*automatchQueueEntry{host, joiner},
		Queue:       queue,
		MapName:     mapName,
		MapID:       mapID,
		RelayRegion: host.RelayRegion,
		State:       automatchWaitingHost,
		CreatedAt:   time.Now(),
	}
	host.MatchID = matchID
	joiner.MatchID = matchID
	delete(mm.queue, host.UserID)
	delete(mm.queue, joiner.UserID)
	mm.matches[matchID] = match

	if mm.game != nil && mm.game.MatchHistory() != nil {
		// Remember map/queue now: the matchmaking entry is deleted as soon as
		// the players leave the match, which happens before they report the
		// result.
		mm.game.MatchHistory().RecordPendingMatch(MatchResult{
			MatchID:       int64(matchID),
			MapID:         mapID,
			MapName:       mapName,
			LeaderboardID: LeaderboardForQueue(queue.ID),
			QueueType:     fmt.Sprintf("automatch2-%d", queue.ID),
			Players:       []int32{host.UserID, joiner.UserID},
			Teams:         []int32{0, 1},
			StartedAt:     time.Now(),
		})
	}

	logger.Printf(
		"automatch2: matched users %d (host poll=%d) and %d (joiner poll=%d) with match %d queue=%d map=%s relay=%q\n",
		host.UserID,
		host.PollID,
		joiner.UserID,
		joiner.PollID,
		matchID,
		queue.ID,
		mapName,
		match.RelayRegion,
	)

	time.AfterFunc(automatchHostTimeout, func() {
		mm.expireMatch(matchID)
	})

	return nil
}

func (mm *MainMatchmaking) expireMatch(matchID int32) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	match, ok := mm.matches[matchID]
	if !ok {
		return
	}
	if match.State == automatchWaitingHost || match.State == automatchHosting {
		logger.Printf(
			"automatch2: match %d timed out waiting for host advertisement (host=%d state=%d)\n",
			matchID,
			match.HostID,
			match.State,
		)
		mm.cancelMatchLocked(match, true, 0)
	}
}

func (mm *MainMatchmaking) matchedResponseLocked(entry *automatchQueueEntry, match *automatchMatch, r *http.Request) (i.A, bool) {
	advertisementID := match.AdvertisementID
	var advertisement Advertisement
	if advertisementID == 0 {
		// Before the host calls /advertisement/host we reserve the match ID as
		// the future advertisement ID. The host client uses this response to
		// create the advertisement with that exact ID.
		advertisementID = match.ID
	} else {
		advertisements := mm.game.Advertisements()
		var ok bool
		advertisement, ok = advertisements.GetAdvertisement(advertisementID)
		if !ok {
			return nil, false
		}
	}
	// The official matched response contains every player in the match, even
	// before the joiner has called /advertisement/join. It also assigns the
	// teams (host 0, joiner 1), while the advertisement peer itself keeps the
	// client-provided team (-1).
	region := match.RelayRegion
	if advertisement != nil {
		if advRegion := advertisement.GetRelayRegion(); advRegion != "" {
			region = advRegion
		}
	}
	battleServer := mm.selectBattleServerLocked(region)
	if battleServer == nil {
		return nil, false
	}
	// Resolve the relay address from the request that is being answered. The
	// advertisement IP is a synthetic virtual address ("/10.0.11.x") used only
	// as a peer identifier; using it here made the client try to resolve it as
	// a real hostname.
	ip := battleServer.ResolveIPv4(r)
	peers := mm.matchPeersLocked(match, advertisementID, ip)
	if peers == nil {
		peers = []i.A{}
	}

	ports := battleServer.EncodePorts()
	for len(ports) < 3 {
		ports = append(ports, 0)
	}
	response := i.A{
		0,
		encodeMatchedAutomatchQueueStatus(match, entry),
		entry.PollID,
		nil,
		match.HostID,
		match.MapName,
		0,
		0,
		0,
		strconv.Itoa(match.Queue.MaxPlayers),
		strconv.Itoa(match.Queue.TeamSize),
		strconv.Itoa(int(match.Queue.LeaderboardID)),
		advertisementID,
		"authtoken",
		battleServer.ResolveIPv4(r),
		ports[0],
		ports[1],
		ports[2],
		battleServer.Region(),
		peers,
		"",
		matchCompletionValue(mm.game.Title()),
	}
	logger.Printf(
		"automatch2: matched polling response user=%d poll=%d match=%d adv=%d pendingHost=%t queue=%d map=%s region=%q peers=%d\n",
		entry.UserID,
		entry.PollID,
		match.ID,
		advertisementID,
		advertisement == nil,
		match.Queue.ID,
		match.MapName,
		battleServer.Region(),
		len(peers),
	)
	return response, true
}

func (mm *MainMatchmaking) matchPeersLocked(match *automatchMatch, advertisementID int32, ip string) []i.A {
	peers := make([]i.A, 0, len(match.Players))
	for _, player := range match.Players {
		user, ok := mm.game.Users().GetUserById(player.UserID)
		if !ok {
			continue
		}
		team := int32(1)
		if player.UserID == match.HostID {
			team = 0
		}
		peer := NewPeer(
			advertisementID,
			ip,
			player.UserID,
			user.GetStatGroupID(),
			-1,
			0,
			team,
		)
		peers = append(peers, peer.Encode())
	}
	return peers
}

func (mm *MainMatchmaking) selectBattleServerLocked(region string) BattleServer {
	battleServers := mm.game.BattleServers()
	if region != "" {
		if battleServer, ok := battleServers.Get(region); ok {
			return battleServer
		}
	}
	for _, battleServer := range battleServers.Iter() {
		return battleServer
	}
	return nil
}

type automatchMapCandidate struct {
	ID   int32
	Name string
}

// normalizeMapName makes the map names sent in vetoMaps comparable with the
// names in automatchMaps.json (e.g. "Arabian Desert.rms" and
// "Arabian Desert" are the same map for veto purposes).
func normalizeMapName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".rms2")
	name = strings.TrimSuffix(name, ".rms")
	name = strings.ReplaceAll(name, "_", " ")
	return strings.Join(strings.Fields(name), " ")
}

func (entry *automatchQueueEntry) vetoesMap(candidate automatchMapCandidate) bool {
	if entry == nil {
		return false
	}
	if slices.Contains(entry.VetoMapKeys, candidate.ID) {
		return true
	}
	candidateName := normalizeMapName(candidate.Name)
	for _, veto := range entry.VetoMapNames {
		if normalizeMapName(veto) == candidateName {
			return true
		}
	}
	return false
}

// selectableMaps returns the maps neither player vetoed. If both players
// covered the whole pool, it preserves the match by relaxing the vetoes:
// first the host's bans, then the joiner's bans, and finally all maps.
func selectableMaps(candidates []automatchMapCandidate, host, joiner *automatchQueueEntry) ([]automatchMapCandidate, bool) {
	allowed := make([]automatchMapCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !host.vetoesMap(candidate) && !joiner.vetoesMap(candidate) {
			allowed = append(allowed, candidate)
		}
	}
	if len(allowed) > 0 {
		return allowed, false
	}

	for _, candidate := range candidates {
		if !host.vetoesMap(candidate) {
			allowed = append(allowed, candidate)
		}
	}
	if len(allowed) > 0 {
		return allowed, true
	}
	for _, candidate := range candidates {
		if !joiner.vetoesMap(candidate) {
			allowed = append(allowed, candidate)
		}
	}
	if len(allowed) > 0 {
		return allowed, true
	}
	return append(allowed, candidates...), true
}

func (mm *MainMatchmaking) chooseMap(queue automatchQueue, host, joiner *automatchQueueEntry) (string, int32) {
	var candidates []automatchMapCandidate
	resources := mm.game.Resources()
	if resources != nil {
		if maps, ok := resources.ArrayFiles()["automatchMaps.json"]; ok && len(maps) > 1 {
			if entries, ok := maps[1].([]any); ok {
				for _, rawEntry := range entries {
					entry, ok := rawEntry.([]any)
					if !ok || len(entry) < 5 {
						continue
					}
					// Entry shape: [mapID, mapName, teams, maxPlayers,
					// leaderboardID, ...].
					maxPlayers, ok := numberToInt(entry[3])
					if !ok || maxPlayers != queue.MaxPlayers {
						continue
					}
					leaderboardID, ok := numberToInt(entry[4])
					if !ok || int32(leaderboardID) != queue.LeaderboardID {
						continue
					}
					mapName, ok := entry[1].(string)
					if !ok {
						continue
					}
					mapID, _ := numberToInt(entry[0])
					candidates = append(candidates, automatchMapCandidate{
						ID:   int32(mapID),
						Name: mapName,
					})
				}
			}
		}
	}
	if len(candidates) == 0 {
		return "Arabia.rms", 1
	}

	allowed, fallback := selectableMaps(candidates, host, joiner)
	if fallback {
		logger.Printf(
			"automatch2: every map in the queue was vetoed (host=%v joiner=%v); falling back to %d map(s)\n",
			host.VetoMapNames,
			joiner.VetoMapNames,
			len(allowed),
		)
	}

	var chosen automatchMapCandidate
	i.WithRng(func(rand *i.RandReader) {
		chosen = allowed[rand.IntN(len(allowed))]
	})
	logger.Printf(
		"automatch2: map selected map=%s id=%d allowed=%d/%d hostVetoes=%v joinerVetoes=%v\n",
		chosen.Name,
		chosen.ID,
		len(allowed),
		len(candidates),
		host.VetoMapNames,
		joiner.VetoMapNames,
	)
	return chosen.Name, chosen.ID
}

func (mm *MainMatchmaking) newPollIDLocked() int32 {
	for {
		var id int32
		i.WithRng(func(rand *i.RandReader) {
			id = int32(rand.Uint32() & 0x7fffffff)
		})
		if id <= 0 {
			continue
		}
		if _, exists := mm.byPoll[id]; exists {
			continue
		}
		return id
	}
}

func (mm *MainMatchmaking) newMatchIDLocked() int32 {
	for {
		var id int32
		i.WithRng(func(rand *i.RandReader) {
			id = int32(rand.Uint32() & 0x7fffffff)
		})
		if id <= 0 {
			continue
		}
		if _, exists := mm.matches[id]; exists {
			continue
		}
		if _, exists := mm.game.Advertisements().GetAdvertisement(id); exists {
			continue
		}
		return id
	}
}

func (mm *MainMatchmaking) removeFromQueueLocked(entry *automatchQueueEntry) {
	if entry == nil {
		return
	}
	if current, ok := mm.queue[entry.UserID]; ok && current == entry {
		delete(mm.queue, entry.UserID)
	}
}

func (mm *MainMatchmaking) removeQueueEntryLocked(entry *automatchQueueEntry) {
	if entry == nil {
		return
	}
	mm.removeFromQueueLocked(entry)
	if current, ok := mm.byUser[entry.UserID]; ok && current == entry {
		delete(mm.byUser, entry.UserID)
	}
	if current, ok := mm.byPoll[entry.PollID]; ok && current == entry {
		delete(mm.byPoll, entry.PollID)
	}
}

func (mm *MainMatchmaking) cancelMatchLocked(match *automatchMatch, requeue bool, exceptUserID int32) {
	if match == nil {
		return
	}
	for _, player := range match.Players {
		player.MatchID = 0
		if player.UserID == exceptUserID {
			mm.removeQueueEntryLocked(player)
			continue
		}
		if requeue && !player.Stopped && player.Session != nil && len(player.MatchTypes) > 0 {
			player.CreatedAt = time.Now()
			mm.queue[player.UserID] = player
			continue
		}
		mm.removeQueueEntryLocked(player)
	}
	delete(mm.matches, match.ID)
}

// AutomatchUnmatchedResponse returns a response with the same shape as the
// official server when no match has been found yet.
func AutomatchUnmatchedResponse(pollID int32, matchTypes []int32) i.A {
	return automatchUnmatchedResponse(pollID, matchTypes, 0)
}

func automatchUnmatchedResponse(pollID int32, matchTypes []int32, waitSeconds int) i.A {
	return i.A{
		0,
		encodeUnmatchedAutomatchQueueStatuses(matchTypes, waitSeconds),
		pollID,
		nil,
		-1,
		"",
		-1,
		-1,
		-1,
		"0",
		"0",
		"-1",
		-1,
		"authtoken",
		"0.0.0.0",
		0,
		0,
		0,
		"",
		[]i.A{},
		nil,
		0,
	}
}

func encodeMatchedAutomatchQueueStatus(match *automatchMatch, entry *automatchQueueEntry) []i.A {
	return []i.A{{
		strconv.Itoa(int(match.Queue.ID)),
		automatchWaitSeconds(entry),
		[]i.A{{int(match.MapID), 1}},
		"0",
	}}
}

func automatchWaitSeconds(entry *automatchQueueEntry) int {
	if entry == nil {
		return 0
	}
	waitSeconds := int(time.Since(entry.CreatedAt).Seconds())
	if waitSeconds < 0 {
		return 0
	}
	return waitSeconds
}

func encodeUnmatchedAutomatchQueueStatuses(matchTypes []int32, waitSeconds int) []i.A {
	if len(matchTypes) == 0 {
		return []i.A{}
	}
	if waitSeconds < 0 {
		waitSeconds = 0
	}
	// The AoE2:DE client asks for several matchTypes but expects the
	// canonical 1v1 Random Map queue in the response. The queue status also
	// carries the currently selected map pool entry.
	return []i.A{{
		strconv.Itoa(int(automatchQueueID)),
		waitSeconds,
		[]i.A{{1, 1}},
		"0",
	}}
}

func cleanMatchTypes(matchTypes []int32) []int32 {
	cleaned := make([]int32, 0, len(matchTypes))
	seen := make(map[int32]struct{}, len(matchTypes))
	for _, matchType := range matchTypes {
		if _, exists := seen[matchType]; exists {
			continue
		}
		seen[matchType] = struct{}{}
		cleaned = append(cleaned, matchType)
	}
	return cleaned
}

func sameParty(a, b *automatchQueueEntry) bool {
	// Solo players commonly send 0 or -1. Only a positive shared party
	// session means that both entries belong to the same party.
	return a.PartySessionID > 0 && a.PartySessionID == b.PartySessionID
}

// compatibleAutomatchQueue currently only supports the AoE2:DE 1v1 Random Map
// queue. Both clients must have at least one known 1v1 matchType enabled.
func compatibleAutomatchQueue(a, b []int32) (automatchQueue, bool) {
	if !containsAnyMatchType(a, automatch1v1Types) || !containsAnyMatchType(b, automatch1v1Types) {
		return automatchQueue{}, false
	}
	return automatchQueue{
		ID:            automatchQueueID,
		MaxPlayers:    automatchQueueMaxPlayers,
		TeamSize:      automatchQueueTeamSize,
		LeaderboardID: automatchQueueLeaderboardID,
	}, true
}

// AoE2:DE has used 20 in the polling request examples and 6 in the canonical
// 1v1 response. Keep both accepted, plus 1 as the older 1v1 alias.
var automatch1v1Types = []int32{20, 6, 1}

func containsAnyMatchType(matchTypes []int32, candidates []int32) bool {
	for _, matchType := range matchTypes {
		for _, candidate := range candidates {
			if matchType == candidate {
				return true
			}
		}
	}
	return false
}

func numberToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	default:
		return 0, false
	}
}

func matchCompletionValue(title string) int {
	// The matched AoE2:DE response uses 225 in the final slot. Other titles
	// use zero when no match is active.
	if title == "age2" {
		return 225
	}
	return 0
}

// IsAutomatchDescription reports whether a host request belongs to the
// matchmaking flow.
func IsAutomatchDescription(description string) bool {
	return description == AutomatchMatchDescription
}
