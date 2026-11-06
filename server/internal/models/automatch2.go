package models

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

const (
	// AutomatchMatchDescription is the description used by the AoE2:DE
	// matchmaking client when it asks the API to create the host
	// advertisement.
	AutomatchMatchDescription = "SESSION_MATCH_KEY"
	// AutomatchAutoMatchDescription is the description the AoE4 client uses for
	// its server-managed matchmaking advertisement.
	AutomatchAutoMatchDescription = "AUTOMATCH"
	automatchGameMessage          = "Automatch2GameMessage"
	automatchHostTimeout          = 45 * time.Second
	// AutomatchAdvertisementMatchType is the advertisement matchtype used by
	// the official AoE2:DE matchmaking host request. AoE4 uses the queue id
	// when the overlay says so (D15).
	AutomatchAdvertisementMatchType uint16 = 19
	// assumeQueueRandomMapSolo is the only accepted --assume-queue value.
	assumeQueueRandomMapSolo = "rm-solo"
)

// AutomatchGameTitles lists the titles that use the automatch2 flow. AoE4 is
// implemented from the captures in docs/matchmaking.md §2.5; AoE1/AoE3/AoM stay
// out until F0 identifies their queue endpoint.
func AutomatchGameTitles() []string {
	return []string{game.AoE2, game.AoE4}
}

// AutomatchSupported reports whether a title uses the automatch2 engine.
func AutomatchSupported(title string) bool {
	return slices.Contains(AutomatchGameTitles(), title)
}

// LeaderboardForQueue maps the legacy AoE2 pool ids to their ladder when the
// catalog is not available (tests, migrations). New code resolves the ladder
// from the queue catalog overlay.
func LeaderboardForQueue(queueID int32) int32 {
	switch queueID {
	case 6, 71, 78, 86, 122, 123:
		return SoloRMRanked
	case 7, 8, 9, 72, 73, 74, 79, 80, 81, 87, 88, 89:
		return 4 // TEAM_RM_RANKED
	case 66:
		return 1 // SOLO_DM_RANKED
	case 67, 68, 69:
		return 2 // TEAM_DM_RANKED
	default:
		return 0
	}
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
	// AoE4 only: the client tells the server whether it already displayed a
	// successful match and which matchID it was given, so a retried poll does
	// not lose the reservation.
	NotifiedAutomatchSuccess bool  `schema:"notifiedAutomatchSuccess"`
	PreviouslyGivenMatchID   int32 `schema:"previouslyGivenMatchId"`
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
	// MatchType and Description are the per-title advertisement values from
	// the queue overlay (D15). AoE2 uses 19/SESSION_MATCH_KEY, AoE4 uses the
	// queue id/AUTOMATCH.
	MatchType   uint16
	Description string
	// Rated is false for co-op/unranked queues and for the opt-in fallback.
	Rated bool
}

type MatchNotification struct {
	Session Session
	Action  string
	Message any
}

type automatchQueueEntry struct {
	Session         Session
	UserID          int32
	PollID          int32
	EstimatedWaitMs int32
	PartySessionID  int64
	MatchTypes      []int32
	RelayRegion     string
	MatchID         int32
	Stopped         bool
	LocalIP         string
	CreatedAt      time.Time
	// QueuedAt is FIFO order and is preserved across requeues (D14).
	QueuedAt time.Time
	RetryAt  time.Time
	// SearchState is 1 while the client is looking for a game and 0 once a
	// match has been assigned (E2: it is not a seconds counter).
	SearchState   int
	RaceInfoKeys  []int32
	RaceInfoRaces []int32
	Options       []string
	FactionIDs    []int32
	VetoMapKeys   []int32
	VetoMapNames  []string
	// unknownLogged deduplicates the unknown-pool metric across the polling
	// loop (one WARN per search, not one every 10 seconds).
	unknownLogged bool
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
	TeamIDs           []int32
	Teams             [][]*automatchQueueEntry
	Queue             *automatchQueue
	MatchType         int32
	MapName           string
	MapID             int32
	RelayRegion       string
	State             automatchMatchState
	AdvertisementID   int32
	PlatformSessionID uint64
	JoinNotified      bool
	CreatedAt         time.Time
}

// automatchPartyGroup is one atomic matchmaking unit. A solo player is a group
// of one; a party is the set of entries whose partySessionID resolves to the
// same session advertisement (D9/E6). A group only participates when every
// advertised member is present in the queue.
type automatchPartyGroup struct {
	PartySessionID int64
	Entries        []*automatchQueueEntry
	QueuedAt       time.Time
	RosterSize     int
	Complete       bool
}

// Matchmaking owns the automatch2 queue. It returns notifications instead of
// importing the wss package, since wss imports this package.
type Matchmaking interface {
	Initialize(game Game)
	Poll(session Session, request *AutomatchPollingRequest, r *http.Request) (i.A, []MatchNotification)
	StopPolling(session Session, request *AutomatchStopPollingRequest) i.A
	UpdateStatus(session Session, request *AutomatchUpdateStatusRequest) i.A
	// UpdateStatusAck returns the per-title ack the handler must use when it
	// cannot bind the request (D5).
	UpdateStatusAck(matchID int32) int
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
	catalog *automatchQueueCatalog
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
	if game == nil {
		mm.catalog = newEmptyQueueCatalog("")
		return
	}
	title := game.Title()
	catalog, err := LoadQueueCatalog(title, game.Resources(), game.LeaderboardDefinitions())
	if err != nil {
		// A broken catalog disables QM for that title instead of pairing the
		// wrong mode. The error is loud and the rest of the server keeps
		// working.
		logger.Warn("automatch2: catalog disabled game=%s: %s", title, err)
		mm.catalog = newEmptyQueueCatalog(title)
		return
	}
	mm.catalog = catalog
	logger.Printf("automatch2: catalog loaded game=%s queues=%d\n", title, catalog.Len())
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
			UserID:          userID,
			PollID:          mm.newPollIDLocked(),
			EstimatedWaitMs: estimatedWaitMillis(),
			CreatedAt:       time.Now(),
			QueuedAt:        time.Now(),
			SearchState:     1,
		}
		mm.byUser[userID] = entry
		mm.byPoll[entry.PollID] = entry
	}

	previousMatchTypes := entry.MatchTypes
	previousPartySessionID := entry.PartySessionID
	previousRelayRegion := entry.RelayRegion
	wasStopped := entry.Stopped
	newSearch := !exists || wasStopped ||
		!slices.Equal(previousMatchTypes, matchTypes) ||
		previousPartySessionID != request.PartySessionID
	if newSearch && entry.MatchID == 0 {
		entry.CreatedAt = time.Now()
		entry.QueuedAt = time.Now()
		entry.EstimatedWaitMs = estimatedWaitMillis()
		entry.unknownLogged = false
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
	entry.RaceInfoKeys = append([]int32(nil), request.RaceInfoKey.Data...)
	entry.RaceInfoRaces = append([]int32(nil), request.RaceInfoRaceID.Data...)
	entry.Options = append([]string(nil), request.Options.Data...)
	entry.FactionIDs = append([]int32(nil), request.FactionIDs.Data...)
	entry.VetoMapKeys = append([]int32(nil), request.VetoMapKey.Data...)
	entry.VetoMapNames = append([]string(nil), request.VetoMaps.Data...)

	if !exists || wasStopped || !slices.Equal(previousMatchTypes, matchTypes) ||
		previousPartySessionID != request.PartySessionID || previousRelayRegion != request.RelayRegion {
		logger.Printf(
			"automatch2: poll user=%d poll=%d estimatedWaitMs=%d call=%d matchTypes=%v party=%d relay=%q profiles=%v races=%v factions=%v vetoMapKey=%v vetoMaps=%v notified=%t previousMatch=%d\n",
			userID,
			entry.PollID,
			entry.EstimatedWaitMs,
			request.CallNum,
			matchTypes,
			request.PartySessionID,
			request.RelayRegion,
			request.RaceInfoProfileID.Data,
			request.RaceInfoRaceID.Data,
			request.FactionIDs.Data,
			request.VetoMapKey.Data,
			request.VetoMaps.Data,
			request.NotifiedAutomatchSuccess,
			request.PreviouslyGivenMatchID,
		)
	}

	// Idempotency: the AoE4 client tells the server which matchID it already
	// received. Re-attach it instead of starting a second search (D12).
	if entry.MatchID == 0 && request.PreviouslyGivenMatchID > 0 {
		if match, ok := mm.matches[request.PreviouslyGivenMatchID]; ok && matchHasUser(match, userID) {
			entry.MatchID = match.ID
			entry.SearchState = 0
			logger.Printf("automatch2: reattached user=%d to match=%d from previouslyGivenMatchId\n", userID, match.ID)
		}
	}

	if entry.MatchID != 0 {
		if match, ok := mm.matches[entry.MatchID]; ok {
			switch match.State {
			case automatchReady:
				if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
					return response, nil
				}
				// The advertisement disappeared. Give every player another
				// chance instead of leaving them waiting forever.
				match.State = automatchHosting
				mm.cancelMatchLocked(match, true, 0)
				entry.MatchID = 0
			case automatchWaitingHost, automatchHosting:
				if entry.UserID == match.HostID {
					if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
						return response, nil
					}
				}
				// The joiner already knows its reserved matchID (slot 12) but
				// waits for the host advertisement before receiving the full
				// matched payload.
				return mm.unmatchedLocked(entry, matchTypes, 0, match.ID), nil
			}
		} else {
			entry.MatchID = 0
		}
	}

	if len(matchTypes) == 0 {
		mm.removeFromQueueLocked(entry)
		return automatchUnmatchedResponse(entry.EstimatedWaitMs, nil, nil, 1, 0), nil
	}

	entry.MatchTypes = matchTypes
	entry.Session = session
	entry.SearchState = 1
	mm.queue[userID] = entry

	if match := mm.tryCreateMatchLocked(entry, matchTypes); match != nil {
		if entry.UserID == match.HostID {
			if response, ok := mm.matchedResponseLocked(entry, match, r); ok {
				return response, nil
			}
		} else {
			return mm.unmatchedLocked(entry, matchTypes, 0, match.ID), nil
		}
	}

	return mm.unmatchedLocked(entry, matchTypes, 1, 0), nil
}

func matchHasUser(match *automatchMatch, userID int32) bool {
	for _, player := range match.Players {
		if player.UserID == userID {
			return true
		}
	}
	return false
}

// tryCreateMatchLocked is the deterministic matching rule of §4.3: resolve the
// polling group against the catalog, then assemble complete groups FIFO until
// the queue is full. It never pairs two known but different queues.
func (mm *MainMatchmaking) tryCreateMatchLocked(entry *automatchQueueEntry, matchTypes []int32) *automatchMatch {
	if mm.catalog == nil {
		return nil
	}
	pollingGroup := mm.groupForEntryLocked(entry, nil)
	if pollingGroup == nil {
		return nil
	}

	for _, queueID := range mm.catalog.Order() {
		queue := mm.catalog.queues[queueID]
		if !groupSupportsQueue(pollingGroup, queue.ID) || !pollingGroup.Complete {
			continue
		}
		groups := mm.groupsForQueueLocked(queue)
		candidates := make([]*automatchPartyGroup, 0, len(groups))
		for _, group := range groups {
			if group == pollingGroup || !group.Complete {
				continue
			}
			if pollingGroup.PartySessionID > 0 && group.PartySessionID == pollingGroup.PartySessionID {
				continue
			}
			candidates = append(candidates, group)
		}
		slices.SortFunc(candidates, func(a, b *automatchPartyGroup) int {
			return a.QueuedAt.Compare(b.QueuedAt)
		})
		selected := []*automatchPartyGroup{pollingGroup}
		total := len(pollingGroup.Entries)
		for _, candidate := range candidates {
			if total == queue.MaxPlayers {
				break
			}
			if total+len(candidate.Entries) <= queue.MaxPlayers {
				selected = append(selected, candidate)
				total += len(candidate.Entries)
			}
		}
		if total != queue.MaxPlayers {
			continue
		}
		players, teamIDs, ok := assignTeams(selected, queue)
		if !ok {
			continue
		}
		host := oldestEntry(players)
		if host == nil {
			return nil
		}
		if match := mm.createMatchLocked(host, players, teamIDs, queue); match != nil {
			return match
		}
	}

	// Opt-in fallback for an unknown pool. It only applies when the polling
	// entry has no known queue; known queues are never rewritten and two known
	// but different modes never pair.
	if !mm.catalog.HasKnownMatchType(entry.MatchTypes) {
		// Unknown pools always leave a trace, with or without the opt-in, so
		// F5 can refresh automatchMaps.json from real client data.
		if !entry.unknownLogged {
			logUnknownMatchTypes(mm.catalog.gameID, entry.MatchTypes)
			entry.unknownLogged = true
		}
		if internalAssumeQueue() == assumeQueueRandomMapSolo {
			return mm.tryAssumeQueueMatchLocked(entry)
		}
	}
	return nil
}

// groupForEntryLocked builds the party group that contains entry. When queue
// is nil the group keeps every entry of the party in the queue.
func (mm *MainMatchmaking) groupForEntryLocked(entry *automatchQueueEntry, queue *automatchQueue) *automatchPartyGroup {
	group := &automatchPartyGroup{
		PartySessionID: entry.PartySessionID,
		RosterSize:     1,
	}
	for _, candidate := range mm.queue {
		if !sameParty(entry, candidate) {
			continue
		}
		if queue != nil && !slices.Contains(candidate.MatchTypes, queue.ID) {
			continue
		}
		group.Entries = append(group.Entries, candidate)
	}
	if len(group.Entries) == 0 {
		return nil
	}
	slices.SortFunc(group.Entries, func(a, b *automatchQueueEntry) int {
		return a.QueuedAt.Compare(b.QueuedAt)
	})
	group.QueuedAt = group.Entries[0].QueuedAt
	if entry.PartySessionID > 0 {
		group.RosterSize = mm.partyRosterSizeLocked(entry.PartySessionID)
	}
	group.Complete = len(group.Entries) == group.RosterSize
	return group
}

func (mm *MainMatchmaking) groupsForQueueLocked(queue *automatchQueue) []*automatchPartyGroup {
	groups := make([]*automatchPartyGroup, 0, len(mm.queue))
	seen := make(map[string]struct{}, len(mm.queue))
	for _, entry := range mm.queue {
		if entry.MatchID != 0 || len(entry.MatchTypes) == 0 || !slices.Contains(entry.MatchTypes, queue.ID) {
			continue
		}
		key := "u:" + strconv.Itoa(int(entry.UserID))
		if entry.PartySessionID > 0 {
			key = "p:" + strconv.FormatInt(entry.PartySessionID, 10)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if group := mm.groupForEntryLocked(entry, queue); group != nil {
			groups = append(groups, group)
		}
	}
	return groups
}

// partyRosterSizeLocked resolves the party size from the session
// advertisement created by the client (E6). If the advertisement is missing
// the unit degrades to one player and a WARN is emitted; completeness is never
// assumed.
func (mm *MainMatchmaking) partyRosterSizeLocked(partySessionID int64) int {
	if partySessionID <= 0 || mm.game == nil || mm.game.Advertisements() == nil {
		return 1
	}
	if partySessionID > int64(^uint32(0)>>1) {
		logger.Warn("automatch2: partySessionID=%d does not fit in int32; treating the unit as solo", partySessionID)
		return 1
	}
	advertisement, ok := mm.game.Advertisements().GetAdvertisement(int32(partySessionID))
	if !ok || advertisement == nil {
		logger.Warn("automatch2: partySessionID=%d has no session advertisement; treating the unit as solo", partySessionID)
		return 1
	}
	peers := advertisement.GetPeers()
	if peers == nil || peers.Len() <= 0 {
		logger.Warn("automatch2: partySessionID=%d has an empty roster; treating the unit as solo", partySessionID)
		return 1
	}
	return peers.Len()
}

func groupSupportsQueue(group *automatchPartyGroup, queueID int32) bool {
	if group == nil || len(group.Entries) == 0 {
		return false
	}
	for _, entry := range group.Entries {
		if !slices.Contains(entry.MatchTypes, queueID) {
			return false
		}
	}
	return true
}

// assignTeams places complete groups into teams without splitting a party. It
// fails when the groups cannot fill the queue exactly, which keeps the
// per-queue guard of §4.3.3.
func assignTeams(groups []*automatchPartyGroup, queue *automatchQueue) ([]*automatchQueueEntry, []int32, bool) {
	if queue == nil || queue.MaxPlayers <= 0 || queue.Teams <= 0 {
		return nil, nil, false
	}
	teamSize := queue.TeamSize
	if teamSize <= 0 {
		teamSize = 1
	}
	sized := make([]int, queue.Teams)
	teams := make([][]*automatchQueueEntry, queue.Teams)
	teamIDs := make([]int32, 0, queue.MaxPlayers)
	players := make([]*automatchQueueEntry, 0, queue.MaxPlayers)
	for _, group := range groups {
		placed := false
		for team := 0; team < queue.Teams; team++ {
			if sized[team]+len(group.Entries) > teamSize {
				continue
			}
			teams[team] = append(teams[team], group.Entries...)
			for range group.Entries {
				teamIDs = append(teamIDs, int32(team))
			}
			sized[team] += len(group.Entries)
			placed = true
			break
		}
		if !placed {
			return nil, nil, false
		}
		players = append(players, group.Entries...)
	}
	if len(players) != queue.MaxPlayers {
		return nil, nil, false
	}
	return players, teamIDs, true
}

func oldestEntry(entries []*automatchQueueEntry) *automatchQueueEntry {
	var oldest *automatchQueueEntry
	for _, entry := range entries {
		if oldest == nil || entry.QueuedAt.Before(oldest.QueuedAt) {
			oldest = entry
		}
	}
	return oldest
}

// tryAssumeQueueMatchLocked pairs two unknown-pool solo entries as an unrated
// 1v1 Random Map. The WARN is the metric required by the DoD; the match is
// forced Rated=false so it cannot contaminate a ladder.
func (mm *MainMatchmaking) tryAssumeQueueMatchLocked(entry *automatchQueueEntry) *automatchMatch {
	queue := mm.assumeQueueLocked()
	if entry.PartySessionID > 0 {
		logger.Warn("automatch2: assume-queue does not group parties; user=%d will wait matchTypes=[%s]", entry.UserID, describeMatchTypes(entry.MatchTypes))
		return nil
	}
	var other *automatchQueueEntry
	for _, candidate := range mm.queue {
		if candidate.UserID == entry.UserID || candidate.MatchID != 0 || len(candidate.MatchTypes) == 0 {
			continue
		}
		if candidate.PartySessionID > 0 {
			continue
		}
		if mm.catalog.HasKnownMatchType(candidate.MatchTypes) {
			continue
		}
		if other == nil || candidate.QueuedAt.Before(other.QueuedAt) {
			other = candidate
		}
	}
	if other == nil {
		return nil
	}
	players := []*automatchQueueEntry{entry, other}
	slices.SortFunc(players, func(a, b *automatchQueueEntry) int {
		return a.QueuedAt.Compare(b.QueuedAt)
	})
	logger.Warn(
		"automatch2: assume-queue pairing unrated game=%s queue=%s leftMatchTypes=[%s] rightMatchTypes=[%s] teams=%d players=%d",
		mm.catalog.gameID,
		assumeQueueRandomMapSolo,
		describeMatchTypes(entry.MatchTypes),
		describeMatchTypes(other.MatchTypes),
		queue.Teams,
		queue.MaxPlayers,
	)
	return mm.createMatchLocked(players[0], players, []int32{0, 1}, queue)
}

func (mm *MainMatchmaking) assumeQueueLocked() *automatchQueue {
	queue := assumeQueueFallback(mm.catalog.gameID)
	// Reuse the maps of the canonical 1v1 Random Map pool when it exists so
	// the fallback still picks a map the client knows.
	if canonical, ok := mm.catalog.Queue(6); ok {
		queue.Maps = append([]automatchMapCandidate(nil), canonical.Maps...)
	}
	return queue
}

func internalAssumeQueue() string {
	return i.AutomatchAssumeQueue
}

func (mm *MainMatchmaking) unmatchedLocked(entry *automatchQueueEntry, matchTypes []int32, state int, matchID int32) i.A {
	return automatchUnmatchedResponse(entry.EstimatedWaitMs, matchTypes, entry.RaceInfoRaces, state, matchID)
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
	// A party stops as one unit: leaving one member out of the queue would
	// leave the others searching for a roster that can never complete.
	if entry.PartySessionID > 0 {
		for _, candidate := range mm.byUser {
			if candidate.UserID == userID || candidate.PartySessionID != entry.PartySessionID ||
				candidate.MatchID != 0 {
				continue
			}
			candidate.Stopped = true
			mm.removeQueueEntryLocked(candidate)
		}
	}
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
	return i.A{0, mm.UpdateStatusAck(request.MatchID)}
}

func (mm *MainMatchmaking) UpdateStatusAck(matchID int32) int {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if match, ok := mm.matches[matchID]; ok && match.Queue != nil {
		return match.Queue.UpdateStatusAck
	}
	_, ack := automatchTitleDefaults(gameTitle(mm.game))
	return ack
}

func gameTitle(g Game) string {
	if g == nil {
		return ""
	}
	return g.Title()
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
			return mm.reservationLocked(match, battleServer), true
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
	return mm.reservationLocked(match, battleServer), true
}

func (mm *MainMatchmaking) reservationLocked(match *automatchMatch, battleServer BattleServer) AutomatchHostReservation {
	reservation := AutomatchHostReservation{
		MatchID:           match.ID,
		BattleServer:      battleServer,
		MapName:           match.MapName,
		MaxPlayers:        uint8(match.Queue.MaxPlayers),
		PlatformSessionID: match.PlatformSessionID,
		MatchType:         AutomatchAdvertisementMatchType,
		Description:       AutomatchMatchDescription,
		Rated:             match.Queue.Rated,
	}
	if match.Queue != nil {
		reservation.MaxPlayers = match.Queue.AdvertisementMaxPlayers()
		reservation.MatchType = match.Queue.AdvertisementMatchType(AutomatchAdvertisementMatchType)
		if gameTitle(mm.game) == game.AoE4 {
			reservation.Description = AutomatchAutoMatchDescription
		}
	}
	return reservation
}

// PrepareHostAdvertisement resolves a pending match from the advertisement ID
// that the AoE2:DE/AoE4 host client uses in its first /advertisement/update
// call. The automatch host does not call /advertisement/host: it assumes the
// server has already allocated the advertisement and only updates its fields.
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
	return mm.reservationLocked(match, battleServer), true
}

// SetPlatformSessionID remembers the platform session that the host created
// for a match. The host announces it before it creates the server-side
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
		"automatch2: host advertisement ready host=%d match=%d adv=%d joiners=%d alreadyNotified=%t queue=%d\n",
		hostID,
		match.ID,
		advertisementID,
		len(match.Players)-1,
		match.JoinNotified,
		match.MatchType,
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
	ports := encodeRelayPorts(battleServer)

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
		peers := mm.matchPeersLocked(match, ip)
		message := encodeAutomatchGameMessageLocked(match, player, ip, ports, peers)
		notifications = append(notifications, MatchNotification{
			Session: player.Session,
			Action:  automatchGameMessage,
			Message: message,
		})
		logger.Printf(
			"automatch2: notifying joiner user=%d poll=%d match=%d host=%d adv=%d map=%s relay=%q peers=%d queue=%d teams=%d players=%d\n",
			player.UserID,
			player.PollID,
			match.ID,
			match.HostID,
			advertisementID,
			match.MapName,
			ip,
			len(peers),
			match.MatchType,
			match.Queue.Teams,
			match.Queue.MaxPlayers,
		)
	}
	return notifications
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
	partySessionID := entry.PartySessionID
	if entry.MatchID != 0 {
		if match, found := mm.matches[entry.MatchID]; found {
			// Only put the other players back in the queue while the host
			// advertisement is still being prepared. Once the match is ready
			// the others are already joining/playing and must not be silently
			// requeued.
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
	// A party leaves atomically: a member dropping out cancels the unit
	// instead of leaving the other members searching alone (§4.3.1).
	if partySessionID > 0 {
		for _, candidate := range mm.byUser {
			if candidate.UserID == userID || candidate.PartySessionID != partySessionID {
				continue
			}
			candidate.Stopped = true
			mm.removeQueueEntryLocked(candidate)
			logger.Printf("automatch2: removed party member user=%d party=%d after user=%d left\n",
				candidate.UserID, partySessionID, userID)
		}
	}
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

func (mm *MainMatchmaking) createMatchLocked(host *automatchQueueEntry, players []*automatchQueueEntry, teamIDs []int32, queue *automatchQueue) *automatchMatch {
	if queue == nil || len(players) != queue.MaxPlayers || len(queue.Maps) == 0 {
		// Per-queue guard (§4.3.3): a 4v4 queue must never create a 2-player
		// lobby waiting for joiners that will never arrive.
		logger.Printf(
			"automatch2: refusing match queue=%d players=%d want=%d maps=%d\n",
			queueID(queue),
			len(players),
			queueMaxPlayers(queue),
			len(queueMaps(queue)),
		)
		return nil
	}
	mapName, mapID, ok := mm.chooseMap(queue, players)
	if !ok {
		logger.Printf("automatch2: no map available for queue=%d; not creating match\n", queue.ID)
		return nil
	}
	matchID := mm.newMatchIDLocked()
	teams := make([][]*automatchQueueEntry, queue.Teams)
	for index, player := range players {
		team := int32(0)
		if index < len(teamIDs) {
			team = teamIDs[index]
		}
		if team >= 0 && int(team) < len(teams) {
			teams[team] = append(teams[team], player)
		}
	}
	match := &automatchMatch{
		ID:          matchID,
		HostID:      host.UserID,
		HostPollID:  host.PollID,
		Players:     players,
		TeamIDs:     append([]int32(nil), teamIDs...),
		Teams:       teams,
		Queue:       queue,
		MatchType:   queue.ID,
		MapName:     mapName,
		MapID:       mapID,
		RelayRegion: host.RelayRegion,
		CreatedAt:   time.Now(),
	}
	match.State = automatchWaitingHost
	for _, player := range players {
		player.MatchID = matchID
		player.SearchState = 0
		delete(mm.queue, player.UserID)
	}
	mm.matches[matchID] = match

	if mm.game != nil && mm.game.MatchHistory() != nil {
		// Remember map/queue now: the matchmaking entry is deleted as soon as
		// the players leave the match, which happens before they report the
		// result.
		playerIDs := make([]int32, 0, len(players))
		for _, player := range players {
			playerIDs = append(playerIDs, player.UserID)
		}
		mm.game.MatchHistory().RecordPendingMatch(MatchResult{
			MatchID:       int64(matchID),
			MapID:         mapID,
			MapName:       mapName,
			MaxPlayers:    int32(queue.MaxPlayers),
			MatchType:     queue.ID,
			QueueID:       queue.ID,
			LeaderboardID: queue.RatedLeaderboard(),
			Rated:         queue.Rated,
			QueueType:     fmt.Sprintf("automatch2-%d", queue.ID),
			Players:       playerIDs,
			Teams:         teamIDs,
			StartedAt:     time.Now(),
		})
	}

	logger.Printf(
		"automatch2: matched match=%d queue=%d mode=%q host=%d players=%v teams=%v map=%s relay=%q rated=%t\n",
		matchID,
		queue.ID,
		queue.Note,
		host.UserID,
		collectPlayerIDs(players),
		teamIDs,
		mapName,
		match.RelayRegion,
		queue.Rated,
	)

	time.AfterFunc(automatchHostTimeout, func() {
		mm.expireMatch(matchID)
	})

	return match
}

func queueID(queue *automatchQueue) int32 {
	if queue == nil {
		return 0
	}
	return queue.ID
}

func queueMaxPlayers(queue *automatchQueue) int {
	if queue == nil {
		return 0
	}
	return queue.MaxPlayers
}

func queueMaps(queue *automatchQueue) []automatchMapCandidate {
	if queue == nil {
		return nil
	}
	return queue.Maps
}

func collectPlayerIDs(players []*automatchQueueEntry) []int32 {
	ids := make([]int32, 0, len(players))
	for _, player := range players {
		ids = append(ids, player.UserID)
	}
	return ids
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
	ip := battleServer.ResolveIPv4(r)
	peers := mm.matchPeersLocked(match, ip)
	if peers == nil {
		peers = []i.A{}
	}
	ports := encodeRelayPorts(battleServer)
	response := encodeMatchedResponse(automatchMatchedParams{
		EstimatedWaitMs: entry.EstimatedWaitMs,
		MatchTypes:      entry.MatchTypes,
		RaceIDs:         entry.RaceInfoRaces,
		HostProfileID:   match.HostID,
		MapName:         match.MapName,
		MapID:           match.MapID,
		Teams:           match.Queue.Teams,
		Players:         match.Queue.MaxPlayers,
		QueueID:         match.MatchType,
		MatchID:         advertisementID,
		IP:              ip,
		Ports:           ports,
		Region:          battleServer.Region(),
		Peers:           peers,
		CompletionValue: match.Queue.MatchCompletionValue,
	})
	logger.Printf(
		"automatch2: matched polling response user=%d poll=%d match=%d adv=%d pendingHost=%t queue=%d map=%s region=%q peers=%d teams=%d players=%d\n",
		entry.UserID,
		entry.PollID,
		match.ID,
		advertisementID,
		advertisement == nil,
		match.MatchType,
		match.MapName,
		battleServer.Region(),
		len(peers),
		match.Queue.Teams,
		match.Queue.MaxPlayers,
	)
	return response, true
}

func encodeRelayPorts(battleServer BattleServer) []int {
	encoded := battleServer.EncodePorts()
	ports := make([]int, 0, len(encoded)+3)
	for _, raw := range encoded {
		if port, ok := numberToInt(raw); ok {
			ports = append(ports, port)
		}
	}
	for len(ports) < 3 {
		ports = append(ports, 0)
	}
	return ports
}

// matchPeersLocked encodes the positional peer rows of the matched polling
// response and the wss message. E4 shows teamID=-1 for every peer: the real
// team assignment travels in match.Teams and in the advertisement slots.
func (mm *MainMatchmaking) matchPeersLocked(match *automatchMatch, ip string) []i.A {
	peers := make([]i.A, 0, len(match.Players))
	for _, player := range match.Players {
		user, ok := mm.game.Users().GetUserById(player.UserID)
		if !ok {
			continue
		}
		peer := NewPeer(
			match.ID,
			ip,
			player.UserID,
			user.GetStatGroupID(),
			-1,
			0,
			-1,
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
	return selectableMapsFor(candidates, []*automatchQueueEntry{host, joiner})
}

// selectableMapsFor generalizes the veto resolution to N players: first the
// maps nobody vetoed, then the maps at least one player allows, then all maps.
func selectableMapsFor(candidates []automatchMapCandidate, players []*automatchQueueEntry) ([]automatchMapCandidate, bool) {
	allowed := make([]automatchMapCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		vetoed := false
		for _, player := range players {
			if player.vetoesMap(candidate) {
				vetoed = true
				break
			}
		}
		if !vetoed {
			allowed = append(allowed, candidate)
		}
	}
	if len(allowed) > 0 {
		return allowed, false
	}

	// Relax the vetoes player by player, preserving the historical order
	// (host first, then joiner) so a party of N keeps a deterministic map.
	for _, player := range players {
		for _, candidate := range candidates {
			if !player.vetoesMap(candidate) {
				allowed = append(allowed, candidate)
			}
		}
		if len(allowed) > 0 {
			return allowed, true
		}
	}
	return append(allowed, candidates...), true
}

func (mm *MainMatchmaking) chooseMap(queue *automatchQueue, players []*automatchQueueEntry) (string, int32, bool) {
	if queue == nil || len(queue.Maps) == 0 {
		// No fallback outside the pool: choosing a map the client does not
		// know is worse than not creating the match (D2/D7).
		return "", 0, false
	}
	allowed, fallback := selectableMapsFor(queue.Maps, players)
	if fallback {
		logger.Printf(
			"automatch2: every map in queue=%d was vetoed; falling back to %d map(s)\n",
			queue.ID,
			len(allowed),
		)
	}
	if len(allowed) == 0 {
		return "", 0, false
	}
	var chosen automatchMapCandidate
	i.WithRng(func(rand *i.RandReader) {
		chosen = allowed[rand.IntN(len(allowed))]
	})
	logger.Printf(
		"automatch2: map selected queue=%d map=%s id=%d allowed=%d/%d\n",
		queue.ID,
		chosen.Name,
		chosen.ID,
		len(allowed),
		len(queue.Maps),
	)
	return chosen.Name, chosen.ID, true
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

// estimatedWaitMillis is the value of polling response slot 2. The client
// renders it as an H:MM:SS estimated wait (e.g. 180000 renders 00:03:00 and
// 567932821 renders 157:45:32), so the official service's tokens observed in
// the captures (180000, 33587, 1001, 1267, 5679) are really wait estimates in
// milliseconds. The LAN server cannot predict a wait, so it reports 0, which
// the client shows as 00:00:00. If a predictor is ever added (F5 metrics),
// this is the single point to change.
func estimatedWaitMillis() int32 {
	return 0
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
		player.SearchState = 1
		if player.UserID == exceptUserID {
			mm.removeQueueEntryLocked(player)
			continue
		}
		if requeue && !player.Stopped && player.Session != nil && len(player.MatchTypes) > 0 {
			// Preserve QueuedAt so a failed host does not send the players to
			// the back of the FIFO (D14).
			player.RetryAt = time.Now()
			mm.queue[player.UserID] = player
			continue
		}
		mm.removeQueueEntryLocked(player)
	}
	delete(mm.matches, match.ID)
}

// AutomatchUnmatchedResponse returns a response with the same shape as the
// official server when no match has been found yet.
func AutomatchUnmatchedResponse(estimatedWaitMs int32, matchTypes []int32) i.A {
	return automatchUnmatchedResponse(estimatedWaitMs, matchTypes, nil, 1, 0)
}

// automatchUnmatchedResponse mirrors the official unmatched body. Slot 2 is
// the estimated wait in milliseconds (rendered by the client as H:MM:SS, see
// estimatedWaitMillis).
func automatchUnmatchedResponse(estimatedWaitMs int32, matchTypes []int32, raceIDs []int32, state int, matchID int32) i.A {
	assigned := int32(-1)
	if matchID != 0 {
		assigned = matchID
	}
	return i.A{
		0,
		encodeUnmatchedAutomatchQueueStatuses(matchTypes, raceIDs, state),
		estimatedWaitMs,
		nil,
		-1,
		"",
		-1,
		-1,
		-1,
		"0",
		"0",
		"-1",
		assigned,
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

// encodeUnmatchedAutomatchQueueStatuses emits one status per requested
// matchType, in the same order, echoing raceInfoRaceID in the map slot (E2).
func encodeUnmatchedAutomatchQueueStatuses(matchTypes []int32, raceIDs []int32, state int) []i.A {
	if len(matchTypes) == 0 {
		return []i.A{}
	}
	statuses := make([]i.A, 0, len(matchTypes))
	for index, matchType := range matchTypes {
		raceID := int32(0)
		if index < len(raceIDs) {
			raceID = raceIDs[index]
		}
		statuses = append(statuses, i.A{
			strconv.Itoa(int(matchType)),
			state,
			i.A{i.A{int(raceID), 1}},
			"0",
		})
	}
	return statuses
}

type automatchMatchedParams struct {
	EstimatedWaitMs int32
	MatchTypes      []int32
	RaceIDs         []int32
	HostProfileID   int32
	MapName         string
	MapID           int32
	Teams           int
	Players         int
	QueueID         int32
	MatchID         int32
	IP              string
	Ports           []int
	Region          string
	Peers           []i.A
	CompletionValue int
}

// encodeMatchedResponse builds the positional matched response captured in
// docs/matchmaking.md §2.2/§2.5.
func encodeMatchedResponse(p automatchMatchedParams) i.A {
	ports := p.Ports
	for len(ports) < 3 {
		ports = append(ports, 0)
	}
	if p.Peers == nil {
		p.Peers = []i.A{}
	}
	return i.A{
		0,
		encodeUnmatchedAutomatchQueueStatuses(p.MatchTypes, p.RaceIDs, 0),
		p.EstimatedWaitMs,
		nil,
		p.HostProfileID,
		p.MapName,
		0,
		0,
		p.MapID,
		strconv.Itoa(p.Teams),
		strconv.Itoa(p.Players),
		strconv.Itoa(int(p.QueueID)),
		p.MatchID,
		"authtoken",
		p.IP,
		ports[0],
		ports[1],
		ports[2],
		p.Region,
		p.Peers,
		"",
		p.CompletionValue,
	}
}

// encodeAutomatchGameMessageLocked builds the inner 15-field positional array
// of Automatch2GameMessage (D16). The wss layer prepends
// [0, "Automatch2GameMessage", recipientProfileID].
func encodeAutomatchGameMessageLocked(match *automatchMatch, recipient *automatchQueueEntry, ip string, ports []int, peers []i.A) i.A {
	raceID := int32(0)
	if len(recipient.RaceInfoRaces) > 0 {
		raceID = recipient.RaceInfoRaces[0]
	}
	if peers == nil {
		peers = []i.A{}
	}
	if len(ports) < 3 {
		ports = append(ports, 0, 0, 0)
	}
	return i.A{
		match.HostID,
		match.ID,
		match.MapName,
		raceID,
		"0",
		"0",
		strconv.Itoa(match.Queue.Teams),
		strconv.Itoa(match.Queue.MaxPlayers),
		"authtoken",
		ip,
		ports[0],
		ports[1],
		ports[2],
		peers,
		"",
	}
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

// IsAutomatchDescription reports whether a host request belongs to the
// matchmaking flow.
func IsAutomatchDescription(description string) bool {
	return description == AutomatchMatchDescription || description == AutomatchAutoMatchDescription
}
