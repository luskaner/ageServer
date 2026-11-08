package models

import (
	"sort"
	"strconv"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

type LeaderboardDefinitions interface {
	Initialize(gameId string, leaderboards i.A)
	AvatarStatDefinitions() AvatarStatDefinitions
	// LadderDefinitions returns the ladder id to name mapping of the title.
	LadderDefinitions() map[int32]string
	HasLadderDefinitions() bool
	HasLadder(id int32) bool
}

type MainLeaderboardDefinitions struct {
	avatarStatDefinitions *MainAvatarStatDefinitions
	ladders               map[int32]string
	hasLadders            bool
}

func (l *MainLeaderboardDefinitions) Initialize(gameId string, leaderboards i.A) {
	l.avatarStatDefinitions = newAvatarStatDefinitions(leaderboards)
	l.ladders = newLadderDefinitions(gameId, leaderboards)
	l.hasLadders = len(l.ladders) > 0
}

func (l *MainLeaderboardDefinitions) AvatarStatDefinitions() AvatarStatDefinitions {
	if l == nil {
		return nil
	}
	return l.avatarStatDefinitions
}

func (l *MainLeaderboardDefinitions) LadderDefinitions() map[int32]string {
	if l == nil {
		return nil
	}
	return l.ladders
}

func (l *MainLeaderboardDefinitions) HasLadderDefinitions() bool {
	return l != nil && l.hasLadders
}

func (l *MainLeaderboardDefinitions) HasLadder(id int32) bool {
	if l == nil || !l.hasLadders {
		return false
	}
	_, ok := l.ladders[id]
	return ok
}

// newAvatarStatDefinitions extracts the avatar stats catalog. The index is 8
// for every supported title; a missing or malformed list leaves the
// definitions nil instead of panicking (age1 has no leaderboards.json).
func newAvatarStatDefinitions(leaderboards i.A) *MainAvatarStatDefinitions {
	if len(leaderboards) <= 8 {
		return nil
	}
	avatarStats, ok := leaderboards[8].(i.A)
	if !ok {
		return nil
	}
	l := &MainAvatarStatDefinitions{
		nameToId: make(map[string]int32, len(avatarStats)),
		idToName: make(map[int32]string, len(avatarStats)),
	}
	for _, rawAvatarStat := range avatarStats {
		avatarStat, ok := rawAvatarStat.(i.A)
		if !ok || len(avatarStat) < 2 {
			continue
		}
		id, idOK := numberToInt(avatarStat[0])
		name, nameOK := avatarStat[1].(string)
		if !idOK || !nameOK {
			continue
		}
		l.nameToId[name] = int32(id)
		l.idToName[int32(id)] = name
	}
	return l
}

// ladderDefinitionsIndex is the slot of the ladder catalog, which varies by
// title: age2 keeps it at index 2 and age3/age4/athens at index 3.
func ladderDefinitionsIndex(gameId string) int {
	switch gameId {
	case game.AoE2:
		return 2
	case game.AoE3, game.AoE4, game.AoM:
		return 3
	default:
		return -1
	}
}

// newLadderDefinitions parses [id, name, ...] rows. Repeated names are valid
// (age2 has two RBW_El_Reinado ladders), so the map is keyed by the numeric id
// parsed from JSON, never by name.
func newLadderDefinitions(gameId string, leaderboards i.A) map[int32]string {
	index := ladderDefinitionsIndex(gameId)
	if index < 0 || len(leaderboards) <= index {
		return nil
	}
	rawLadders, ok := leaderboards[index].(i.A)
	if !ok {
		return nil
	}
	ladders := make(map[int32]string, len(rawLadders))
	for _, rawLadder := range rawLadders {
		ladder, ok := rawLadder.(i.A)
		if !ok || len(ladder) < 2 {
			continue
		}
		id, idOK := numberToInt(ladder[0])
		name, nameOK := ladder[1].(string)
		if !idOK || !nameOK || id < 0 {
			continue
		}
		ladders[int32(id)] = name
	}
	return ladders
}

// LadderIDs exposes the parsed ids sorted, which is what the overlay loader
// and the tests use to validate a queue.
func (l *MainLeaderboardDefinitions) LadderIDs() []int32 {
	if l == nil {
		return nil
	}
	ids := make([]int32, 0, len(l.ladders))
	for id := range l.ladders {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// DefaultLeaderboardFor is the ladder a title uses when the client omits
// leaderboard_id. age1 queries ladder 1 (its Quick Match ELO) and age2 the 1v1
// Random Map ranked ladder (3). Leaderboard 0 means "not rated" everywhere
// else; the defaults below are the best documented mapping until F0 confirms
// each title with a capture.
func DefaultLeaderboardFor(gameId string) int32 {
	switch gameId {
	case game.AoE1:
		return 1
	case game.AoE2:
		return SoloRMRanked
	case game.AoE3, game.AoE4, game.AoM:
		return 1
	default:
		return SoloRMRanked
	}
}

// LadderName renders a ladder id for logs (the client always uses ids).
func LadderName(definitions LeaderboardDefinitions, id int32) string {
	if definitions == nil {
		return strconv.Itoa(int(id))
	}
	if name, ok := definitions.LadderDefinitions()[id]; ok {
		return name
	}
	return strconv.Itoa(int(id))
}

type AvatarStatDefinitions interface {
	GetIdByName(name string) (id int32, ok bool)
	GetNameById(id int32) (name string, ok bool)
}

type MainAvatarStatDefinitions struct {
	nameToId map[string]int32
	idToName map[int32]string
}

func (as *MainAvatarStatDefinitions) GetIdByName(name string) (id int32, ok bool) {
	if as == nil {
		return 0, false
	}
	id, ok = as.nameToId[name]
	return
}

func (as *MainAvatarStatDefinitions) GetNameById(id int32) (name string, ok bool) {
	if as == nil {
		return "", false
	}
	name, ok = as.idToName[id]
	return
}
