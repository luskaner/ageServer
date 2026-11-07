package communityEvent

import (
	"encoding/json"
	"fmt"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

const (
	// seasonEventType is the community-event type the AoE4 client uses to
	// decide whether a ranked season is active. Without an event of this type
	// whose [start, end] window contains "now" the ranked queue is disabled
	// ("no active season") and the search button cannot be clicked.
	seasonEventType = 6
	// seasonEventFarFuture mirrors the sentinel the official service sends in
	// the fourth field of a season event (2035-06-25).
	seasonEventFarFuture int64 = 2066425200
	// seasonHistoryDepth is how many seasons are published: the active one plus
	// the previous one, which the ranked pane shows as "last season".
	seasonHistoryDepth = 2
)

// aoe4CommunityEvents builds the getAvailableCommunityEvents response for
// AoE4. The season definitions (rank table, placement matches, decay and the
// leaderboards each queue scores on) come from communityEvents.json; the
// window and the season number are generated so the ranked queue is always
// playable and the ladder resets on the same boundary the client shows.
//
// The response has the seven blocks of the official service:
//
//	[0, events, eventChallenges, ?, seasonLeaderboards, seasonLeaderboardLinks, divisions]
//
// Block 4 (seasonLeaderboards) declares one slot per season queue and block 5
// (seasonLeaderboardLinks) maps each slot to the base leaderboards it scores;
// without block 5 the client cannot resolve the season's ladder and reports
// that there is no active season.
func aoe4CommunityEvents(definitions i.A, now time.Time) i.A {
	empty := i.A{0, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}}
	seasons, ok := seasonDefinitions(definitions)
	if !ok || len(seasons) == 0 {
		return empty
	}
	current := models.CurrentRankedSeason(now)
	events := make(i.A, 0, len(seasons)*seasonHistoryDepth)
	links := make(i.A, 0, len(seasons)*seasonHistoryDepth)
	seasonLeaderboards := make(i.A, 0, len(seasons)*seasonHistoryDepth)
	for depth := 0; depth < seasonHistoryDepth; depth++ {
		season := current
		for step := 0; step < depth; step++ {
			if season.Number <= 1 {
				season = models.RankedSeason{}
				break
			}
			season = season.Previous()
		}
		if season.Number <= 0 {
			continue
		}
		baseIndex := (season.Number - 1) * len(seasons)
		for index, definition := range seasons {
			metadata, err := json.Marshal(map[string]any{"leaderboard": definition.leaderboard})
			if err != nil {
				continue
			}
			name := fmt.Sprintf("Season %d - %s", season.Number, definition.name)
			id := int32(1000 + baseIndex + index)
			seasonIndex := baseIndex + index + 1
			events = append(events, i.A{
				name,
				season.Start.Unix(),
				season.End.Unix(),
				seasonEventFarFuture,
				id,
				seasonEventType,
				fmt.Sprintf("Season %d: %s", season.Number, definition.description),
				string(metadata),
			})
			links = append(links, i.A{
				id,
				seasonIndex,
				name + "_" + definition.leaderboardLink,
				1,
				2,
			})
			for _, base := range definition.baseLeaderboards {
				seasonLeaderboards = append(seasonLeaderboards, i.A{
					seasonIndex,
					base,
					1,
					-1,
					id,
				})
			}
		}
	}
	return i.A{0, events, i.A{}, i.A{}, links, seasonLeaderboards, i.A{}}
}

type seasonDefinition struct {
	kind            string
	name            string
	leaderboardLink string
	description     string
	leaderboard     any
	// baseLeaderboards are the ladders this season queue scores on. They are
	// the ids the client resolves through the season leaderboard block.
	baseLeaderboards []int32
}

// seasonDefinitions reads the [0, [definition...]] resource. A missing or
// malformed resource yields no seasons, which keeps the previous behaviour
// (ranked disabled) instead of serving a half-built season.
func seasonDefinitions(resource i.A) ([]seasonDefinition, bool) {
	if len(resource) < 2 {
		return nil, false
	}
	rawDefinitions, ok := resource[1].([]any)
	if !ok {
		return nil, false
	}
	definitions := make([]seasonDefinition, 0, len(rawDefinitions))
	for _, raw := range rawDefinitions {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		link, _ := entry["leaderboardLink"].(string)
		description, _ := entry["description"].(string)
		leaderboard, hasLeaderboard := entry["leaderboard"]
		if name == "" || link == "" || !hasLeaderboard {
			continue
		}
		kind, _ := entry["kind"].(string)
		definitions = append(definitions, seasonDefinition{
			kind:             kind,
			name:             name,
			leaderboardLink:  link,
			description:      description,
			leaderboard:      leaderboard,
			baseLeaderboards: baseLeaderboards(entry["baseLeaderboards"]),
		})
	}
	return definitions, true
}

// baseLeaderboards parses the ladder ids a season queue scores on.
func baseLeaderboards(raw any) []int32 {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	ids := make([]int32, 0, len(list))
	for _, value := range list {
		number, ok := value.(float64)
		if !ok || int32(number) <= 0 {
			continue
		}
		ids = append(ids, int32(number))
	}
	return ids
}
