package router

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	athensEvent "github.com/luskaner/ageServer/server/internal/models/athens/routes/game/communityEvent"
)

// TestCommunityAge1_EmptyCatalogueIsNotAnError covers the §7.2 age1 case: the
// title ships neither leaderboards.json nor achievements.json, and both endpoints
// must answer empty arrays with result.code 0 (never 500, never null).
func TestCommunityAge1_EmptyCatalogueIsNotAnError(t *testing.T) {
	g := newTestGame(t, game.AoE1)
	// newTestGame injects a placeholder leaderboards.json for every title so the
	// game can boot; removing it reproduces the real age1 resource set.
	if fake, ok := g.Resources().(*fakeResources); ok {
		delete(fake.arrayFiles, "leaderboards.json")
		delete(fake.arrayFiles, "achievements.json")
	}
	h := (&Community{}).InitializeRoutes(game.AoE1, nil)

	for path, key := range map[string]string{
		"/community/leaderboard/getAvailableLeaderboards": "leaderboards",
		"/community/achievement/getAvailableAchievements": "achievementDefs",
		"/community/news/getNews":                         "news",
	} {
		body := decodeBody(t, doCommunityRequest(t, h, path+"?title=age1", g))
		result, ok := body["result"].(map[string]any)
		if !ok {
			t.Errorf("%s: result is not an object: %v", path, body)
			continue
		}
		if code, _ := result["code"].(float64); int(code) != 0 {
			t.Errorf("%s: result.code = %v, want 0", path, result["code"])
		}
		list, ok := body[key].([]any)
		if !ok {
			t.Errorf("%s: %s is not an array: %v", path, key, body[key])
			continue
		}
		if len(list) != 0 {
			t.Errorf("%s: %s = %v, want empty", path, key, body[key])
		}
	}
}

// TestCommunityAge1_LeaderboardCatalogueKeepsEveryBlock checks that the age1
// answer still carries the full catalogue shape with empty blocks, so a client
// parses it exactly like the other titles.
func TestCommunityAge1_LeaderboardCatalogueKeepsEveryBlock(t *testing.T) {
	g := newTestGame(t, game.AoE1)
	if fake, ok := g.Resources().(*fakeResources); ok {
		delete(fake.arrayFiles, "leaderboards.json")
	}
	h := (&Community{}).InitializeRoutes(game.AoE1, nil)
	body := decodeBody(t, doCommunityRequest(t, h, "/community/leaderboard/getAvailableLeaderboards", g))
	for _, key := range []string{"leaderboards", "matchTypes", "races", "factions", "leaderboardRegions"} {
		list, ok := body[key].([]any)
		if !ok {
			t.Errorf("%s is not an array: %v", key, body[key])
			continue
		}
		if len(list) != 0 {
			t.Errorf("%s = %v, want empty", key, body[key])
		}
	}
}

// TestCommunityAthens_NewsUsesImageURL checks the AoM news mapping of §1.a: the
// image URL lives in position 5 and the body is empty.
func TestCommunityAthens_NewsUsesImageURL(t *testing.T) {
	g := newTestGame(t, game.AoM)
	fake, ok := g.Resources().(*fakeResources)
	if !ok {
		t.Fatalf("resources are %T", g.Resources())
	}
	fake.news = []i.A{
		{
			float64(34), "Summer 2026", "Headline", nil, "Text", "https://cdn.example.com/a.png",
			"", `{"type":"NEWS","url":"https://example.com/a"}`, "1785430800", "1798480800",
		},
	}
	h := (&Community{}).InitializeRoutes(game.AoM, nil)
	body := decodeBody(t, doCommunityRequest(t, h, "/community/news/getNews?title=athens", g))
	items, _ := body["news"].([]any)
	if len(items) != 1 {
		t.Fatalf("news = %v, want 1", body["news"])
	}
	item, _ := items[0].(map[string]any)
	if image, _ := item["newsimage"].(string); image != "https://cdn.example.com/a.png" {
		t.Fatalf("newsimage = %v, want the AoM image URL", item["newsimage"])
	}
	if newsBody, _ := item["newsbody"].(string); newsBody != "" {
		t.Fatalf("newsbody = %v, want empty for AoM", item["newsbody"])
	}
	if date, _ := item["datevisible"].(float64); int64(date) != 1785430800 {
		t.Fatalf("datevisible = %v, want 1785430800", item["datevisible"])
	}
}

// TestCommunityAoM_CommunityEventsHasThreeBlocks checks the C6 nuance: AoM
// answers communityEvents plus leaderboards and leaderboardMaps, while AoE2 and
// AoE4 answer only communityEvents.
func TestCommunityAoM_CommunityEventsHasThreeBlocks(t *testing.T) {
	athensEvent.Initialize()
	g := newTestGame(t, game.AoM)
	h := (&Community{}).InitializeRoutes(game.AoM, nil)
	body := decodeBody(t, doCommunityRequest(t, h, "/community/CommunityEvent/getAvailableCommunityEvents?title=athens", g))
	events, ok := body["communityEvents"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("communityEvents = %v, want a populated array", body["communityEvents"])
	}
	event, _ := events[0].(map[string]any)
	assertKeys(t, "communityEvents[0]", event,
		"id", "name", "start", "end", "expiryTime", "scheduledstarttime", "scheduledendtime",
		"scheduledexpirytime", "actualstarttime", "actualendtime", "customdata",
		"eventstate", "eventtype")
	// AoM does not publish localizedName.
	if _, present := event["localizedName"]; present {
		t.Error("AoM event carries localizedName, which the official AoM does not send")
	}
	if _, present := body["leaderboards"]; !present {
		t.Error("AoM answer is missing leaderboards")
	}
	if _, present := body["leaderboardMaps"]; !present {
		t.Error("AoM answer is missing leaderboardMaps")
	}
	maps, _ := body["leaderboardMaps"].([]any)
	if len(maps) > 0 {
		mapping, _ := maps[0].(map[string]any)
		assertKeys(t, "leaderboardMaps[0]", mapping,
			"communityevent_id", "communityeventleaderboard_id", "matchtype_id",
			"statgroup_type", "civilization_id")
		if _, present := mapping["race_id"]; present {
			t.Error("leaderboardMaps[0] carries race_id instead of civilization_id")
		}
	}
	leaderboards, _ := body["leaderboards"].([]any)
	if len(leaderboards) > 0 {
		leaderboard, _ := leaderboards[0].(map[string]any)
		assertKeys(t, "leaderboards[0]", leaderboard,
			"communityevent_id", "id", "name", "isranked", "scoringtype")
	}
}

// TestCommunityAoE2_CommunityEventsIsSingleBlock checks the other half of C6:
// AoE2 answers only communityEvents.
func TestCommunityAoE2_CommunityEventsIsSingleBlock(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := (&Community{}).InitializeRoutes(game.AoE2, nil)
	body := decodeBody(t, doCommunityRequest(t, h, "/community/CommunityEvent/getAvailableCommunityEvents", g))
	assertKeys(t, "age2 communityEvents", body, "result", "communityEvents")
	if events, ok := body["communityEvents"].([]any); !ok || len(events) != 0 {
		t.Fatalf("communityEvents = %v, want an empty array", body["communityEvents"])
	}
}

// TestCommunityAoE4_CommunityEventsUsesTheRollingSeason checks that AoE4 serves the
// season events of the game route, in object form.
func TestCommunityAoE4_CommunityEventsUsesTheRollingSeason(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	fake, ok := g.Resources().(*fakeResources)
	if !ok {
		t.Fatalf("resources are %T", g.Resources())
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "age4", "communityEvents.json"))
	if err != nil {
		t.Fatalf("read age4 communityEvents.json: %v", err)
	}
	var decoded i.A
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode communityEvents.json: %v", err)
	}
	fake.arrayFiles["communityEvents.json"] = decoded

	h := (&Community{}).InitializeRoutes(game.AoE4, nil)
	body := decodeBody(t, doCommunityRequest(t, h, "/community/CommunityEvent/getAvailableCommunityEvents?title=age4", g))
	events, ok := body["communityEvents"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("communityEvents = %v, want the rolling season events", body["communityEvents"])
	}
	event, _ := events[0].(map[string]any)
	assertKeys(t, "age4 communityEvents[0]", event,
		"id", "name", "start", "end", "expiryTime", "scheduledstarttime", "scheduledendtime",
		"scheduledexpirytime", "actualstarttime", "actualendtime", "customdata",
		"eventstate", "eventtype", "localizedName")
	// AoE4 uses 6 for a ranked season.
	if eventType, _ := event["eventtype"].(float64); int(eventType) != 6 {
		t.Errorf("eventtype = %v, want 6", event["eventtype"])
	}
	// Only communityEvents is emitted for age4.
	assertKeys(t, "age4 communityEvents", body, "result", "communityEvents")
}

// TestCommunityAthens_AvatarStatsRequireAConnectedUser checks the documented
// limitation of §2.c: an offline profile contributes no avatar stats.
func TestCommunityAthens_AvatarStatsRequireAConnectedUser(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := (&Community{}).InitializeRoutes(game.AoE2, nil)
	body := decodeBody(t, doCommunityRequest(t, h,
		"/community/leaderboard/GetAvatarStatForProfile?profile_names="+url.QueryEscape(`["/steam/76561197984749679"]`), g))
	stats, ok := body["avatarStatsForProfile"].([]any)
	if !ok {
		t.Fatalf("avatarStatsForProfile = %v, want an array", body["avatarStatsForProfile"])
	}
	if len(stats) != 0 {
		t.Fatalf("avatarStatsForProfile = %v, want empty for an offline profile", stats)
	}
}
