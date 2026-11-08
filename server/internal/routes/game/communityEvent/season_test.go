package communityEvent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func bundledSeasonDefinitions(t *testing.T) i.A {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "resources", "responses", "age4", "communityEvents.json"))
	if err != nil {
		t.Fatalf("read communityEvents.json: %v", err)
	}
	var decoded i.A
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode communityEvents.json: %v", err)
	}
	return decoded
}

func TestAoE4CommunityEventsPublishesAnActiveSeason(t *testing.T) {
	definitions := bundledSeasonDefinitions(t)
	now := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	response := aoe4CommunityEvents(definitions, now)
	if len(response) != 7 {
		t.Fatalf("response has %d blocks, want 7", len(response))
	}
	if code, _ := response[0].(int); code != 0 {
		t.Fatalf("result = %v, want 0", response[0])
	}
	events, ok := response[1].([]any)
	if !ok || len(events) != 4*seasonHistoryDepth {
		t.Fatalf("events = %d, want %d (current + previous season)", len(events), 4*seasonHistoryDepth)
	}
	links, ok := response[4].([]any)
	if !ok || len(links) != len(events) {
		t.Fatalf("links = %d, want one per event", len(links))
	}
	// Block 5 maps every season slot to the ladders it scores on. Without it
	// the client cannot resolve the season and reports "no active season".
	seasonLeaderboards, ok := response[5].([]any)
	if !ok || len(seasonLeaderboards) == 0 {
		t.Fatalf("season leaderboards = %#v, want the base ladder mapping", response[5])
	}
	season := models.CurrentRankedSeason(now)
	previous := season.Previous()
	seen := map[string]bool{}
	for index, raw := range events {
		event, ok := raw.(i.A)
		if !ok || len(event) != 8 {
			t.Fatalf("event %d = %#v, want 8 fields", index, raw)
		}
		name, _ := event[0].(string)
		if name == "" {
			t.Fatalf("event %d has no name", index)
		}
		seen[name] = true
		start, _ := event[1].(int64)
		end, _ := event[2].(int64)
		// The active season contains now; the previous one ended before it.
		active := start <= now.Unix() && now.Unix() < end
		finished := end <= now.Unix()
		if !active && !finished {
			t.Fatalf("event %q window [%d, %d) is neither active nor finished", name, start, end)
		}
		if event[5] != seasonEventType {
			t.Fatalf("event %q type = %v, want %d", name, event[5], seasonEventType)
		}
		metadata, ok := event[7].(string)
		if !ok || metadata == "" {
			t.Fatalf("event %q has no metadata", name)
		}
		var decoded struct {
			Leaderboard struct {
				RankLevels struct {
					CustomRanks []any `json:"customRanks"`
				} `json:"rankLevels"`
				PointUpdate struct {
					PlacementMatch int `json:"placementMatch"`
				} `json:"pointUpdate"`
			} `json:"leaderboard"`
		}
		if err := json.Unmarshal([]byte(metadata), &decoded); err != nil {
			t.Fatalf("event %q metadata is not JSON: %v", name, err)
		}
		if len(decoded.Leaderboard.RankLevels.CustomRanks) == 0 || decoded.Leaderboard.PointUpdate.PlacementMatch == 0 {
			t.Fatalf("event %q metadata lost the ranked configuration", name)
		}
		link, ok := links[index].(i.A)
		if !ok || len(link) != 5 {
			t.Fatalf("link %d = %#v, want 5 fields", index, links[index])
		}
		if link[0] != event[4] {
			t.Fatalf("link %d points at event %v, want %v", index, link[0], event[4])
		}
	}
	for _, expected := range []string{
		"Season 19 - Ranked Solo - Crossplay",
		"Season 19 - Ranked Solo - Controller",
		"Season 19 - Ranked Teams - Crossplay",
		"Season 19 - Ranked Teams - Controller",
		"Season 18 - Ranked Solo - Crossplay",
		"Season 18 - Ranked Teams - Controller",
	} {
		if !seen[expected] {
			t.Fatalf("missing season event %q (got %v)", expected, seen)
		}
	}
	if season.Number != 19 {
		t.Fatalf("season number = %d, want 19 for 2026-11-07 with 90-day seasons", season.Number)
	}
	if previous.Number != 18 || !previous.End.Equal(season.Start) {
		t.Fatalf("previous season = %+v, want 18 ending at %s", previous, season.Start)
	}

	// The active solo queue must map to ladder 1 and the team queue to 2/3/4.
	activeEventIDs := map[int32]bool{}
	for _, raw := range events {
		event := raw.(i.A)
		start, _ := event[1].(int64)
		end, _ := event[2].(int64)
		if start <= now.Unix() && now.Unix() < end {
			activeEventIDs[event[4].(int32)] = true
		}
	}
	baseByEvent := map[int32][]int32{}
	for _, raw := range seasonLeaderboards {
		entry, ok := raw.(i.A)
		if !ok || len(entry) != 5 {
			t.Fatalf("season leaderboard row = %#v", raw)
		}
		eventID := entry[4].(int32)
		if activeEventIDs[eventID] {
			baseByEvent[eventID] = append(baseByEvent[eventID], entry[1].(int32))
		}
	}
	var allBases []int32
	for _, bases := range baseByEvent {
		allBases = append(allBases, bases...)
	}
	sort.Slice(allBases, func(a, b int) bool { return allBases[a] < allBases[b] })
	want := []int32{1, 2, 3, 4, 214, 220, 226, 232}
	if len(allBases) != len(want) {
		t.Fatalf("active season base ladders = %v, want %v", allBases, want)
	}
	for index := range want {
		if allBases[index] != want[index] {
			t.Fatalf("active season base ladders = %v, want %v", allBases, want)
		}
	}
}

func TestAoE4CommunityEventsWithoutResourceDisablesRanked(t *testing.T) {
	response := aoe4CommunityEvents(nil, time.Now())
	if len(response) != 7 {
		t.Fatalf("response has %d blocks, want 7", len(response))
	}
	events, ok := response[1].([]any)
	if !ok || len(events) != 0 {
		t.Fatalf("events = %#v, want none", response[1])
	}
}

func TestSeasonDefinitionsRejectsMalformedEntries(t *testing.T) {
	definitions := i.A{0, i.A{
		map[string]any{"name": "", "leaderboardLink": "x", "leaderboard": map[string]any{}},
		map[string]any{"name": "Ranked Solo", "leaderboardLink": "", "leaderboard": map[string]any{}},
		map[string]any{"name": "Ranked Solo", "leaderboardLink": "x"},
		"not an object",
	}}
	parsed, ok := seasonDefinitions(definitions)
	if !ok {
		t.Fatal("seasonDefinitions rejected the resource shape")
	}
	if len(parsed) != 0 {
		t.Fatalf("parsed = %#v, want no usable definitions", parsed)
	}
}
