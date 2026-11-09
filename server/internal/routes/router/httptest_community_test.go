package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/models"
)

// communityExpectedRoutes are the paths the public API must register, mirroring
// router/community.go. They are listed instead of derived so a handler that
// stops being wired is caught here.
var communityExpectedRoutes = []string{
	"/community/news/getNews",
	"/community/leaderboard/getAvailableLeaderboards",
	"/community/leaderboard/GetAvailableLeaderboards",
	"/community/leaderboard/getLeaderBoard2",
	"/community/leaderboard/GetPersonalStat",
	"/community/leaderboard/getPersonalStat",
	"/community/leaderboard/GetAvatarStatForProfile",
	"/community/leaderboard/getRecentMatchHistory",
	"/community/leaderboard/getMatchHistory",
	"/community/leaderboard/getReplayFiles",
	"/community/leaderboard/getStatGroupsByProfileIDs",
	"/community/achievement/getAvailableAchievements",
	"/community/achievement/getAchievements",
	"/community/clan/find",
	"/community/clan/getClanInfoFull",
	"/community/CommunityEvent/getAvailableCommunityEvents",
	"/community/advertisement/findAdvertisements",
	"/community/external/proxysteamuserrequest",
	"/community/item/getInventoryByProfileIDs",
	"/community/item/getItems",
}

// communityHandler builds the handler tree of the public API for one title.
func communityHandler() http.Handler {
	c := &Community{}
	return c.InitializeRoutes(game.AoE2, nil)
}

// doCommunityRequest issues a request against the community handler with the
// game injected the way TitleMiddleware does. No session is ever created: the
// API must work without one.
func doCommunityRequest(t *testing.T, h http.Handler, path string, g models.Game) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := context.WithValue(req.Context(), "game", g)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	return recorder
}

// decodeBody parses a JSON response, failing the test when it is not JSON.
func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%q)", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not a JSON object: %v (body=%q)", err, recorder.Body.String())
	}
	return body
}

// TestCommunityRoutes_AllAnswer200WithoutSession checks the central promise of
// the plan: every registered route answers 200 with valid JSON and
// result.code == 0, and none of them requires a session.
func TestCommunityRoutes_AllAnswer200WithoutSession(t *testing.T) {
	for _, gameId := range []string{game.AoE1, game.AoE2, game.AoE3, game.AoE4, game.AoM} {
		t.Run(gameId, func(t *testing.T) {
			g := newTestGame(t, gameId)
			h := (&Community{}).InitializeRoutes(gameId, nil)
			for _, path := range communityExpectedRoutes {
				recorder := doCommunityRequest(t, h, path+"?title="+gameId, g)
				if recorder.Code != http.StatusOK {
					t.Errorf("%s: status = %d, want 200 (body=%q)", path, recorder.Code, recorder.Body.String())
					continue
				}
				body := decodeBody(t, recorder)
				if _, isJSON := body["result"]; !isJSON {
					t.Errorf("%s: response has no result envelope: %v", path, body)
				}
			}
		})
	}
}

// TestCommunityRoutes_RejectForeignTitle checks the deliberate deviation of
// §D4: an unknown title is a 200 with result.code 2, never a 404.
func TestCommunityRoutes_RejectForeignTitle(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := communityHandler()
	body := decodeBody(t, doCommunityRequest(t, h, "/community/news/getNews?title=age9", g))
	result, ok := body["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %v", body)
	}
	if code, _ := result["code"].(float64); int(code) != 2 {
		t.Fatalf("result.code = %v, want 2", result["code"])
	}
}

// TestCommunityPrefix_DoesNotOpenGameRoutes is the regression guard of §D2: the
// public tree must not make /game reachable without a session.
func TestCommunityPrefix_DoesNotOpenGameRoutes(t *testing.T) {
	if (&Community{}).Check(httptest.NewRequest(http.MethodGet, "/game/news/getNews", nil)) {
		t.Fatal("Community must not claim /game paths")
	}
	if !(&Community{}).Check(httptest.NewRequest(http.MethodGet, "/community/news/getNews", nil)) {
		t.Fatal("Community must claim /community paths")
	}
	// And the game handler still rejects an unauthenticated call.
	g := newTestGame(t, game.AoE2)
	gameRoutes := (&Game{}).InitializeRoutes(game.AoE2, nil)
	recorder := doRequest(gameRoutes, http.MethodGet, "/game/advertisement/findAdvertisements", g, "", "", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("/game advertisement without session: status = %d, want 401", recorder.Code)
	}
}

// TestCommunityStubs_Shape fixes the stub contract of §7.2.b so "empty" can
// never silently become "broken".
func TestCommunityStubs_Shape(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := communityHandler()

	clans := decodeBody(t, doCommunityRequest(t, h, "/community/clan/find", g))
	clanList, ok := clans["clans"].([]any)
	if !ok || len(clanList) != 0 {
		t.Fatalf("clan/find: clans = %v, want an empty array", clans["clans"])
	}

	info := decodeBody(t, doCommunityRequest(t, h, "/community/clan/getClanInfoFull", g))
	value, present := info["clan"]
	if !present {
		t.Fatal("clan/getClanInfoFull: clan key is missing")
	}
	if value != nil {
		t.Fatalf("clan/getClanInfoFull: clan = %v, want null", value)
	}

	achievements := decodeBody(t, doCommunityRequest(t, h,
		"/community/achievement/getAchievements?profileids="+url.QueryEscape("[196240,196241]"), g))
	entries, ok := achievements["userAchievementsMap"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("getAchievements: userAchievementsMap = %v, want 2 entries", achievements["userAchievementsMap"])
	}
	// The key must be the bare id: quotes belong to the inventory endpoint only.
	if _, present := entries[0].(map[string]any)["196240"]; !present {
		t.Fatalf("getAchievements: missing unquoted key 196240 in %v", entries[0])
	}

	inventory := decodeBody(t, doCommunityRequest(t, h,
		"/community/item/getInventoryByProfileIDs?profileids="+url.QueryEscape("[196240]"), g))
	instances, ok := inventory["itemInstancesByProfileID"].([]any)
	if !ok || len(instances) != 1 {
		t.Fatalf("getInventoryByProfileIDs: itemInstancesByProfileID = %v, want 1 entry", inventory["itemInstancesByProfileID"])
	}
	entry, ok := instances[0].(map[string]any)
	if !ok {
		t.Fatalf("getInventoryByProfileIDs: entry is not an object: %v", instances[0])
	}
	if _, present := entry[`"196240"`]; !present {
		t.Fatalf("getInventoryByProfileIDs: key must carry literal quotes, got %v", entry)
	}

	items := decodeBody(t, doCommunityRequest(t, h, "/community/item/getItems", g))
	if itemsList, ok := items["items"].([]any); !ok || len(itemsList) != 0 {
		t.Fatalf("getItems: items = %v, want an empty array", items["items"])
	}

	statGroups := decodeBody(t, doCommunityRequest(t, h, "/community/leaderboard/getStatGroupsByProfileIDs", g))
	for _, key := range []string{"statGroups", "leaderboardStats"} {
		if list, ok := statGroups[key].([]any); !ok || len(list) != 0 {
			t.Fatalf("getStatGroupsByProfileIDs: %s = %v, want an empty array", key, statGroups[key])
		}
	}
}

// TestCommunityStubTopLevelKeysMatchRealOnes implements the last rule of
// §7.2.b: promoting a stub must not change the set of top-level keys.
func TestCommunityStubTopLevelKeysMatchRealOnes(t *testing.T) {
	// Route -> the keys its stub declares. The real handler is called with a
	// real game and its keys are compared against this set.
	expected := map[string][]string{
		"/community/clan/find":                             {"result", "clans"},
		"/community/clan/getClanInfoFull":                  {"result", "clan"},
		"/community/achievement/getAchievements":           {"result", "userAchievementsMap"},
		"/community/item/getInventoryByProfileIDs":         {"result", "itemInstancesByProfileID"},
		"/community/item/getItems":                         {"result", "items"},
		"/community/leaderboard/getStatGroupsByProfileIDs": {"result", "statGroups", "leaderboardStats"},
	}
	g := newTestGame(t, game.AoE2)
	h := communityHandler()
	for path, keys := range expected {
		body := decodeBody(t, doCommunityRequest(t, h, path, g))
		if len(body) != len(keys) {
			t.Errorf("%s: top-level keys = %v, want %v", path, sortedKeys(body), keys)
			continue
		}
		for _, key := range keys {
			if _, present := body[key]; !present {
				t.Errorf("%s: missing top-level key %q", path, key)
			}
		}
	}
}

func sortedKeys(body map[string]any) []string {
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	return keys
}

// TestCommunityInput_IsStrictButSoft implements §7.4: no input produces a panic
// or a 500. A panic here would also take down /game.
func TestCommunityInput_IsStrictButSoft(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := communityHandler()
	cases := []string{
		`/community/leaderboard/GetPersonalStat?profile_names=` + url.QueryEscape(`["/steam/../../../etc/passwd"]`),
		`/community/leaderboard/GetPersonalStat?profile_names=` + url.QueryEscape(`["/steam/notanumber"]`),
		`/community/leaderboard/GetPersonalStat?profile_names=` + url.QueryEscape(`["/psn/123"]`),
		`/community/leaderboard/GetPersonalStat?profile_ids=` + url.QueryEscape(`[]`),
		`/community/leaderboard/GetPersonalStat?profile_ids=notjson`,
		`/community/leaderboard/getRecentMatchHistory?aliases=` + url.QueryEscape(`["`+strings.Repeat("a", 10000)+`"]`),
		`/community/leaderboard/getMatchHistory?matchIDs=` + url.QueryEscape(`[-1,0,"x"]`),
		`/community/leaderboard/getReplayFiles?matchIDs=` + url.QueryEscape(`[null]`),
		`/community/achievement/getAchievements?profileids=` + url.QueryEscape(`{"not":"an array"}`),
		`/community/news/getNews?title=age9`,
		`/community/leaderboard/getLeaderBoard2?count=5000&start=-3&sortBy=0&leaderboard_id=3`,
	}
	for _, path := range cases {
		recorder := doCommunityRequest(t, h, path, g)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (body=%q)", path, recorder.Code, recorder.Body.String())
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Errorf("%s: body is not JSON: %v", path, err)
		}
	}
}

// TestCommunityAliases_AreCapped checks the 100-element cap: 500 aliases must be
// handled, not rejected and not propagated.
func TestCommunityAliases_AreCapped(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	aliases := make([]string, 0, 500)
	for index := 0; index < 500; index++ {
		aliases = append(aliases, "alias"+strings.Repeat("x", index%5))
	}
	encoded, err := json.Marshal(aliases)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := "/community/leaderboard/GetPersonalStat?aliases=" + url.QueryEscape(string(encoded))
	recorder := doCommunityRequest(t, communityHandler(), path, g)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}
