package models

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

// automatchQueue is one family of the client map pool plus the values that a
// pool entry cannot express (ladder, rated flag, per-title acks). See
// docs/matchmaking.md §4.2.
type automatchQueue struct {
	ID         int32
	Teams      int
	MaxPlayers int
	TeamSize   int
	// LeaderboardID is the ladder that receives the rating of this queue. It is
	// 0 for unrated queues.
	LeaderboardID int32
	Rated         bool
	// MatchCompletionValue is the final slot of the matched polling response
	// and of the wss message (225 in age2, 1 in age4, 0 until validated).
	MatchCompletionValue int
	// UpdateStatusAck is the second slot of /automatch2/updateStatus (age2 0,
	// age4 30).
	UpdateStatusAck int
	// MaxPlayersOverride is used by the host advertisement instead of
	// MaxPlayers (age4 advertises 16 players even for a 2-player queue).
	MaxPlayersOverride int
	// MatchTypeIsAdvertisement makes the host advertisement carry the queue id
	// as matchtype instead of the per-title default (AoE4 uses 860).
	MatchTypeIsAdvertisement bool
	Maps                     []automatchMapCandidate
	Note                     string
}

func (q *automatchQueue) AdvertisementMaxPlayers() uint8 {
	players := q.MaxPlayers
	if q.MaxPlayersOverride > 0 {
		players = q.MaxPlayersOverride
	}
	if players < 0 {
		players = 0
	}
	if players > 255 {
		return 255
	}
	return uint8(players)
}

func (q *automatchQueue) AdvertisementMatchType(defaultMatchType uint16) uint16 {
	if q.MatchTypeIsAdvertisement && q.ID > 0 && q.ID <= 65535 {
		return uint16(q.ID)
	}
	return defaultMatchType
}

// RatedLeaderboard returns the ladder only when the queue is rated. The
// history store treats leaderboard 0 as "play but do not score".
func (q *automatchQueue) RatedLeaderboard() int32 {
	if !q.Rated {
		return 0
	}
	return q.LeaderboardID
}

// automatchQueueCatalog is the pool+overlay view of one title. A title without
// a pool has no Quick Match: the engine never falls back to 1v1 Random Map on
// its own.
type automatchQueueCatalog struct {
	gameID string
	order  []int32
	queues map[int32]*automatchQueue
}

func newEmptyQueueCatalog(gameID string) *automatchQueueCatalog {
	return &automatchQueueCatalog{
		gameID: gameID,
		queues: make(map[int32]*automatchQueue),
	}
}

func (c *automatchQueueCatalog) Queue(id int32) (*automatchQueue, bool) {
	if c == nil {
		return nil, false
	}
	queue, ok := c.queues[id]
	return queue, ok
}

// QueueForMatchTypes resolves the first queue of the catalog present in the
// client list. catalog order is ascending pool id so the result is stable and
// does not depend on the order the client sent.
func (c *automatchQueueCatalog) QueueForMatchTypes(matchTypes []int32) (*automatchQueue, bool) {
	if c == nil {
		return nil, false
	}
	for _, id := range c.order {
		if slices.Contains(matchTypes, id) {
			return c.queues[id], true
		}
	}
	return nil, false
}

// Compatible returns the catalog queue both lists share, or false. Two known
// but different queues are never paired: that is the D1 contract.
func (c *automatchQueueCatalog) Compatible(a, b []int32) (*automatchQueue, bool) {
	if c == nil {
		return nil, false
	}
	for _, id := range c.order {
		if slices.Contains(a, id) && slices.Contains(b, id) {
			return c.queues[id], true
		}
	}
	return nil, false
}

// HasKnownMatchType reports whether at least one of the ids belongs to the
// catalog. It distinguishes "unknown pool" (eligible for the opt-in fallback)
// from "known but different mode" (never paired).
func (c *automatchQueueCatalog) HasKnownMatchType(matchTypes []int32) bool {
	_, ok := c.QueueForMatchTypes(matchTypes)
	return ok
}

func (c *automatchQueueCatalog) Len() int {
	if c == nil {
		return 0
	}
	return len(c.order)
}

func (c *automatchQueueCatalog) Order() []int32 {
	if c == nil {
		return nil
	}
	return append([]int32(nil), c.order...)
}

// LoadQueueCatalog builds the catalog from automatchMaps.json grouped by its
// fifth field and overlays automatchQueues.json on top. A missing pool is not
// an error: it simply means that title has no Quick Match. A malformed pool or
// overlay is an error, because silently pairing the wrong mode is worse than
// not pairing at all.
func LoadQueueCatalog(gameID string, resources Resources, definitions LeaderboardDefinitions) (*automatchQueueCatalog, error) {
	return loadQueueCatalogForGame(gameID, resources, definitions)
}

func loadQueueCatalogForGame(gameID string, resources Resources, definitions LeaderboardDefinitions) (*automatchQueueCatalog, error) {
	catalog := newEmptyQueueCatalog(gameID)
	if resources == nil {
		return catalog, nil
	}
	arrayFiles := resources.ArrayFiles()
	rawPool, ok := arrayFiles["automatchMaps.json"]
	if !ok || rawPool == nil {
		// No pool for this title: no Quick Match.
		return catalog, nil
	}
	if err := parseAutomatchPool(gameID, rawPool, catalog); err != nil {
		return nil, err
	}
	if overlay, ok := arrayFiles["automatchQueues.json"]; ok && overlay != nil {
		if err := parseAutomatchOverlay(gameID, overlay, catalog, definitions); err != nil {
			return nil, err
		}
	}
	catalog.sort()
	return catalog, nil
}

func (c *automatchQueueCatalog) sort() {
	c.order = make([]int32, 0, len(c.queues))
	for id := range c.queues {
		c.order = append(c.order, id)
	}
	sort.Slice(c.order, func(i, j int) bool { return c.order[i] < c.order[j] })
}

func parseAutomatchPool(gameID string, raw i.A, catalog *automatchQueueCatalog) error {
	root, ok := any(raw).([]any)
	if !ok || len(root) < 2 {
		return fmt.Errorf("automatch2/%s: automatchMaps.json: unexpected root shape", gameID)
	}
	entries, ok := root[1].([]any)
	if !ok {
		return fmt.Errorf("automatch2/%s: automatchMaps.json: missing entry list", gameID)
	}
	type poolShape struct {
		teams   int
		players int
	}
	shapes := make(map[int32]poolShape)
	for index, rawEntry := range entries {
		entry, ok := rawEntry.([]any)
		if !ok || len(entry) < 5 {
			continue
		}
		mapID, ok := numberToInt(entry[0])
		if !ok {
			continue
		}
		mapName, _ := entry[1].(string)
		teams, okTeams := numberToInt(entry[2])
		players, okPlayers := numberToInt(entry[3])
		poolID, okPool := numberToInt(entry[4])
		if !okTeams || !okPlayers || !okPool || poolID <= 0 || teams <= 0 || players <= 0 {
			continue
		}
		id := int32(poolID)
		shape, exists := shapes[id]
		if !exists {
			shape = poolShape{teams: teams, players: players}
			shapes[id] = shape
			catalog.queues[id] = &automatchQueue{
				ID:         id,
				Teams:      teams,
				MaxPlayers: players,
				TeamSize:   teamSizeFor(teams, players),
			}
		} else if shape.teams != teams || shape.players != players {
			return fmt.Errorf(
				"automatch2/%s: pool %d is not homogeneous in (teams, players): first was (%d,%d), entry %d (%q) has (%d,%d)",
				gameID, id, shape.teams, shape.players, index, mapName, teams, players,
			)
		}
		catalog.queues[id].Maps = append(catalog.queues[id].Maps, automatchMapCandidate{
			ID:   int32(mapID),
			Name: mapName,
		})
	}
	if len(catalog.queues) == 0 {
		return fmt.Errorf("automatch2/%s: automatchMaps.json has no usable pool", gameID)
	}
	for id, queue := range catalog.queues {
		completion, ack := automatchTitleDefaults(gameID)
		queue.MatchCompletionValue = completion
		queue.UpdateStatusAck = ack
		if len(queue.Maps) == 0 {
			return fmt.Errorf("automatch2/%s: pool %d has no maps", gameID, id)
		}
	}
	return nil
}

func teamSizeFor(teams, players int) int {
	if teams <= 0 {
		return 1
	}
	size := players / teams
	if size <= 0 {
		return 1
	}
	return size
}

func parseAutomatchOverlay(gameID string, raw i.A, catalog *automatchQueueCatalog, definitions LeaderboardDefinitions) error {
	root, ok := any(raw).([]any)
	if !ok || len(root) < 2 {
		return fmt.Errorf("automatch2/%s: automatchQueues.json: unexpected root shape", gameID)
	}
	entries, ok := root[1].([]any)
	if !ok {
		return fmt.Errorf("automatch2/%s: automatchQueues.json: missing entry list", gameID)
	}
	seen := make(map[int32]struct{})
	for index, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return fmt.Errorf("automatch2/%s: automatchQueues.json entry %d is not an object", gameID, index)
		}
		matchTypes, err := overlayMatchTypes(entry)
		if err != nil {
			return fmt.Errorf("automatch2/%s: automatchQueues.json entry %d: %w", gameID, index, err)
		}
		if len(matchTypes) == 0 {
			return fmt.Errorf("automatch2/%s: automatchQueues.json entry %d has no matchTypes", gameID, index)
		}
		teams, players := 0, 0
		for _, matchType := range matchTypes {
			if _, duplicate := seen[matchType]; duplicate {
				return fmt.Errorf("automatch2/%s: matchType %d is declared twice in automatchQueues.json", gameID, matchType)
			}
			seen[matchType] = struct{}{}
			queue, exists := catalog.queues[matchType]
			if !exists {
				return fmt.Errorf("automatch2/%s: matchType %d from automatchQueues.json is not in automatchMaps.json", gameID, matchType)
			}
			if teams == 0 {
				teams, players = queue.Teams, queue.MaxPlayers
			} else if queue.Teams != teams || queue.MaxPlayers != players {
				return fmt.Errorf(
					"automatch2/%s: overlay entry %d groups matchType %d (%d,%d) with a different family (%d,%d)",
					gameID, index, matchType, queue.Teams, queue.MaxPlayers, teams, players,
				)
			}
		}
		ladder, _ := overlayInt(entry, "ladder")
		if ladder < 0 {
			return fmt.Errorf("automatch2/%s: overlay entry %d has a negative ladder", gameID, index)
		}
		rated, _ := entry["rated"].(bool)
		if rated && ladder <= 0 {
			return fmt.Errorf("automatch2/%s: overlay entry %d is rated but has no ladder", gameID, index)
		}
		if rated && definitions != nil && definitions.HasLadderDefinitions() && !definitions.HasLadder(int32(ladder)) {
			return fmt.Errorf("automatch2/%s: overlay entry %d uses ladder %d, which leaderboards.json does not define", gameID, index, ladder)
		}
		completion, hasCompletion := overlayInt(entry, "matchCompletionValue")
		ack, hasAck := overlayInt(entry, "updateStatusAck")
		maxOverride, _ := overlayInt(entry, "maxPlayersOverride")
		matchTypeIsAdvertisement, _ := entry["matchTypeIsAdvertisement"].(bool)
		note, _ := entry["note"].(string)
		for _, matchType := range matchTypes {
			queue := catalog.queues[matchType]
			queue.LeaderboardID = int32(ladder)
			queue.Rated = rated
			queue.MatchTypeIsAdvertisement = matchTypeIsAdvertisement
			queue.Note = note
			if hasCompletion {
				queue.MatchCompletionValue = completion
			}
			if hasAck {
				queue.UpdateStatusAck = ack
			}
			if maxOverride > 0 {
				queue.MaxPlayersOverride = maxOverride
			}
		}
	}
	return nil
}

func overlayMatchTypes(entry map[string]any) ([]int32, error) {
	raw, ok := entry["matchTypes"]
	if !ok {
		return nil, errors.New("missing matchTypes")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("matchTypes is not an array")
	}
	matchTypes := make([]int32, 0, len(list))
	for _, value := range list {
		number, ok := numberToInt(value)
		if !ok || number <= 0 {
			return nil, fmt.Errorf("invalid matchType %v", value)
		}
		matchTypes = append(matchTypes, int32(number))
	}
	return matchTypes, nil
}

func overlayInt(entry map[string]any, key string) (int, bool) {
	raw, ok := entry[key]
	if !ok {
		return 0, false
	}
	return numberToInt(raw)
}

// automatchTitleDefaults are the values captured for each title. Overlay
// entries can override them per queue.
func automatchTitleDefaults(gameID string) (completion int, ack int) {
	switch gameID {
	case game.AoE2:
		return 225, 0
	case game.AoE4:
		return 1, 30
	default:
		return 0, 0
	}
}

// assumeQueueFallback builds the opt-in unrated 1v1 Random Map queue. It is
// only used when neither side has a matchType in the catalog, so two known but
// different modes are never paired.
func assumeQueueFallback(gameID string) *automatchQueue {
	completion, ack := automatchTitleDefaults(gameID)
	return &automatchQueue{
		ID:                   6, // canonical 1v1 Random Map pool id
		Teams:                2,
		MaxPlayers:           2,
		TeamSize:             1,
		LeaderboardID:        0,
		Rated:                false,
		MatchCompletionValue: completion,
		UpdateStatusAck:      ack,
		Note:                 "assume-queue=rm-solo",
	}
}

// describeMatchTypes renders ids for structured logs/metrics.
func describeMatchTypes(matchTypes []int32) string {
	parts := make([]string, 0, len(matchTypes))
	for _, id := range matchTypes {
		parts = append(parts, strconv.Itoa(int(id)))
	}
	return strings.Join(parts, ",")
}

// logUnknownMatchTypes records the D1 metric: every unknown pool must leave a
// trace so F5 can refresh automatchMaps.json with real data.
func logUnknownMatchTypes(gameID string, matchTypes []int32) {
	if len(matchTypes) == 0 {
		return
	}
	logger.Warn(
		"automatch2: unknown matchTypes game=%s matchTypes=[%s] (no pool family; not paired)",
		gameID,
		describeMatchTypes(matchTypes),
	)
}
