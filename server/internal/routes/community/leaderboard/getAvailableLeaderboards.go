package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// Index positions of the ladder catalogue inside leaderboards.json. The layout
// is the same for age2, age3, age4 and athens (verified empirically against all
// four resources), while age1 ships no leaderboards.json at all:
//
//	0 result code
//	1 [ladderID, matchtypeID, statgroup_type, race_id, -1]  -> leaderboardmap
//	2 [id, name, isranked]                                  -> leaderboards
//	3 [id, name, ...]                                       -> matchTypes
//	4 [id, name, faction_id, locstringid]                   -> races
//	5 factions (empty except athens)
//	6 [id, name, locstringid]                               -> leaderboardRegions
//
// Note that leaderboards[] always comes from slot 2. models.ladderDefinitionsIndex
// returns 3 for age3/age4/athens because it serves a different purpose; it must
// not be reused here (§1.b).
const (
	ladderMapsIndex            = 1
	ladderDefinitionsIndex     = 2
	matchTypesIndex            = 3
	racesIndex                 = 4
	factionsIndex              = 5
	leaderboardRegionsIndex    = 6
	minLeaderboardResourceSize = 7
)

// LeaderboardMap is one leaderboardmap entry. The field is civilization_id, not
// race_id: the wiki is outdated and the live service renamed it (C3).
type LeaderboardMap struct {
	MatchTypeID    int64 `json:"matchtype_id"`
	StatGroupType  int64 `json:"statgroup_type"`
	CivilizationID int64 `json:"civilization_id"`
}

// Leaderboard is one ladder of the catalogue.
type Leaderboard struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	IsRanked       int64            `json:"isranked"`
	LeaderboardMap []LeaderboardMap `json:"leaderboardmap"`
}

// MatchType is one matchTypes entry. LocalizedName is only emitted when the
// resource carries it: age4 does, age2 has null in that slot (C3).
type MatchType struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LocStringID   int64  `json:"locstringid"`
	LocalizedName string `json:"localizedName,omitempty"`
}

// Race is one races entry.
type Race struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FactionID     int64  `json:"faction_id"`
	LocStringID   int64  `json:"locstringid"`
	LocalizedName string `json:"localizedName,omitempty"`
}

// Faction is one factions entry, non-empty only for Athens.
type Faction struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	LocStringID int64  `json:"locstringid"`
	FactionID   int64  `json:"faction_id"`
}

// Region is one leaderboardRegions entry.
type Region struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LocStringID   int64  `json:"locstringid"`
	LocalizedName string `json:"localizedName,omitempty"`
}

// CatalogueResponse is the body of getAvailableLeaderboards.
type CatalogueResponse struct {
	Result             shared.Result `json:"result"`
	Leaderboards       []Leaderboard `json:"leaderboards"`
	MatchTypes         []MatchType   `json:"matchTypes"`
	Races              []Race        `json:"races"`
	Factions           []Faction     `json:"factions"`
	LeaderboardRegions []Region      `json:"leaderboardRegions"`
}

// GetAvailableLeaderboards returns the full catalogue of ladders, match types,
// races, factions and regions of the title.
//
// The shape is the one the live service returns, not the two-array minimum the
// first draft of the plan proposed: leaderboardmap carries civilization_id and
// the secondary blocks are part of the contract (§1.b, C3).
func GetAvailableLeaderboards(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	resource := game.Resources().ArrayFiles()["leaderboards.json"]
	response := emptyCatalogue()
	if len(resource) < minLeaderboardResourceSize {
		// age1 has no leaderboards.json. Empty arrays with result OK, never a
		// 500 or null (§1.b, §9).
		i.JSON(&w, response)
		return
	}

	// leaderboardmap must be attached to the ladder it belongs to. The resource
	// does not nest it, so it is grouped by ladder id first.
	mapsByLadder := make(map[int64][]LeaderboardMap)
	for _, raw := range shared.RowsAt(resource, ladderMapsIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		ladderID, ok := shared.Int64At(entry, 0)
		if !ok {
			continue
		}
		matchTypeID, _ := shared.Int64At(entry, 1)
		statGroupType, _ := shared.Int64At(entry, 2)
		civilizationID, _ := shared.Int64At(entry, 3)
		mapsByLadder[ladderID] = append(mapsByLadder[ladderID], LeaderboardMap{
			MatchTypeID:    matchTypeID,
			StatGroupType:  statGroupType,
			CivilizationID: civilizationID,
		})
	}

	for _, raw := range shared.RowsAt(resource, ladderDefinitionsIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		ladderID, _ := shared.Int64At(entry, 0)
		isRanked, _ := shared.Int64At(entry, 2)
		maps := mapsByLadder[ladderID]
		if maps == nil {
			maps = []LeaderboardMap{}
		}
		response.Leaderboards = append(response.Leaderboards, Leaderboard{
			ID:             ladderID,
			Name:           shared.StringAt(entry, 1),
			IsRanked:       isRanked,
			LeaderboardMap: maps,
		})
	}

	for _, raw := range shared.RowsAt(resource, matchTypesIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		id, _ := shared.Int64At(entry, 0)
		response.MatchTypes = append(response.MatchTypes, MatchType{
			ID:            id,
			Name:          shared.StringAt(entry, 1),
			LocStringID:   -1,
			LocalizedName: shared.StringAt(entry, 4),
		})
	}

	for _, raw := range shared.RowsAt(resource, racesIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		id, _ := shared.Int64At(entry, 0)
		factionID, _ := shared.Int64At(entry, 2)
		response.Races = append(response.Races, Race{
			ID:            id,
			Name:          shared.StringAt(entry, 1),
			FactionID:     factionID,
			LocStringID:   -1,
			LocalizedName: shared.StringAt(entry, 3),
		})
	}

	for _, raw := range shared.RowsAt(resource, factionsIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		id, _ := shared.Int64At(entry, 0)
		factionID, _ := shared.Int64At(entry, 2)
		response.Factions = append(response.Factions, Faction{
			ID:          id,
			Name:        shared.StringAt(entry, 1),
			LocStringID: -1,
			FactionID:   factionID,
		})
	}

	for _, raw := range shared.RowsAt(resource, leaderboardRegionsIndex) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		id, _ := shared.Int64At(entry, 0)
		response.LeaderboardRegions = append(response.LeaderboardRegions, Region{
			ID:            id,
			Name:          shared.StringAt(entry, 1),
			LocStringID:   -1,
			LocalizedName: shared.StringAt(entry, 2),
		})
	}

	i.JSON(&w, response)
}

// emptyCatalogue is the response with every block present but empty.
func emptyCatalogue() CatalogueResponse {
	return CatalogueResponse{
		Result:             shared.OK(),
		Leaderboards:       []Leaderboard{},
		MatchTypes:         []MatchType{},
		Races:              []Race{},
		Factions:           []Faction{},
		LeaderboardRegions: []Region{},
	}
}
