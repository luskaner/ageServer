package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
	advertisementShared "github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

// communityContractGame builds a game whose resources carry enough data for the
// real encoders to produce a full object, so the contract tests can inspect
// every field instead of an empty shell.
//
// The resources are patched after CreateMainGame: Resources.Initialize runs as
// part of the game construction and would overwrite anything set in the setup
// callback.
func communityContractGame(t *testing.T) models.Game {
	t.Helper()
	g := newTestGame(t, game.AoE2)
	fake, ok := g.Resources().(*fakeResources)
	if !ok {
		t.Fatalf("resources are %T, want *fakeResources", g.Resources())
	}
	// leaderboards.json with all the blocks the catalogue endpoint reads.
	fake.arrayFiles["leaderboards.json"] = i.A{
		0,
		i.A{
			i.A{float64(1), float64(2), float64(1), float64(-1), float64(-1)},
			i.A{float64(3), float64(6), float64(1), float64(-1), float64(-1)},
		},
		i.A{
			i.A{float64(1), "SOLO_DM_RANKED", float64(1)},
			i.A{float64(3), "SOLO_RM_RANKED", float64(1)},
		},
		i.A{
			i.A{float64(2), "1V1", float64(0), float64(0), nil, float64(1), float64(1), "", "", float64(1)},
		},
		i.A{
			i.A{float64(0), "Armenians", float64(0), nil},
		},
		i.A{},
		i.A{
			i.A{float64(0), "Europe", nil},
		},
		i.A{},
		i.A{
			i.A{float64(1), "STAT_NUM_WINS", nil, float64(4), float64(0)},
		},
		i.A{},
	}
	// achievements.json: [code, [[8 fields]...], signature]
	fake.arrayFiles["achievements.json"] = i.A{
		0,
		i.A{
			i.A{float64(1), "ACH_ONE", nil, nil, "", float64(1789983650), float64(0), float64(-1)},
			i.A{float64(2), "ACH_DELETED", nil, nil, "", float64(1789983650), float64(1), float64(-1)},
		},
		"signature",
	}
	// news equivalent: 10 positional fields per item, ascending by id.
	fake.news = []i.A{
		{
			float64(1), "first", "First title", nil, "First text", "https://example.com/1.jpg",
			"First body", `{"type":"NEWS"}`, "1000", "2000",
		},
		{
			float64(2), "second", "Second title", nil, "Second text", "https://example.com/2.jpg",
			"Second body", `{"type":"NEWS"}`, "3000", "4000",
		},
	}
	return g
}

func communityContractRequest(t *testing.T, h http.Handler, g models.Game, path string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := context.WithValue(req.Context(), "game", g)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200 (body=%q)", path, recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: body is not JSON: %v", path, err)
	}
	return body
}

// assertKeys verifies that obj has exactly the given keys, catching both a
// missing field and an accidentally added one.
func assertKeys(t *testing.T, label string, obj map[string]any, keys ...string) {
	t.Helper()
	if len(obj) != len(keys) {
		t.Errorf("%s: %d keys, want %d: got %v", label, len(obj), len(keys), sortedKeys(obj))
	}
	for _, key := range keys {
		if _, present := obj[key]; !present {
			t.Errorf("%s: missing key %q", label, key)
		}
	}
}

// TestCommunityContract_News checks the 11 news fields and the datevisible
// ordering (§1.a, C9).
func TestCommunityContract_News(t *testing.T) {
	g := communityContractGame(t)
	body := communityContractRequest(t, communityHandler(), g, "/community/news/getNews")
	items, ok := body["news"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("news = %v, want 2 items", body["news"])
	}
	first, _ := items[0].(map[string]any)
	assertKeys(t, "news[0]", first,
		"id", "hidden", "newscategory", "newstitle_locstringid", "newssubtitle_locstringid",
		"newstext_locstringid", "newsimage", "newsbody", "metadata", "datevisible", "expirytime")
	// id 2 has the newer datevisible, so it must come first.
	if id, _ := first["id"].(float64); int(id) != 2 {
		t.Fatalf("news[0].id = %v, want 2 (datevisible descending)", first["id"])
	}
	if got, _ := first["datevisible"].(float64); int64(got) != 3000 {
		t.Fatalf("news[0].datevisible = %v, want 3000", first["datevisible"])
	}
	if hidden, _ := first["hidden"].(float64); int(hidden) != 0 {
		t.Fatalf("news[0].hidden = %v, want 0", first["hidden"])
	}
	if loc, _ := first["newstitle_locstringid"].(float64); int(loc) != -1 {
		t.Fatalf("news[0].newstitle_locstringid = %v, want -1", first["newstitle_locstringid"])
	}
}

// TestCommunityContract_AvailableLeaderboards checks the full catalogue shape,
// including civilization_id and the blocks the first draft omitted.
func TestCommunityContract_AvailableLeaderboards(t *testing.T) {
	g := communityContractGame(t)
	body := communityContractRequest(t, communityHandler(), g, "/community/leaderboard/getAvailableLeaderboards")
	assertKeys(t, "catalogue", body, "result", "leaderboards", "matchTypes", "races", "factions", "leaderboardRegions")

	leaderboards, _ := body["leaderboards"].([]any)
	if len(leaderboards) != 2 {
		t.Fatalf("leaderboards = %v, want 2", body["leaderboards"])
	}
	ladder, _ := leaderboards[0].(map[string]any)
	assertKeys(t, "leaderboards[0]", ladder, "id", "name", "isranked", "leaderboardmap")
	maps, _ := ladder["leaderboardmap"].([]any)
	if len(maps) != 1 {
		t.Fatalf("leaderboardmap = %v, want 1", ladder["leaderboardmap"])
	}
	entry, _ := maps[0].(map[string]any)
	assertKeys(t, "leaderboardmap[0]", entry, "matchtype_id", "statgroup_type", "civilization_id")
}

// TestCommunityContract_Achievements checks the catalogue fields and that
// deleted entries are skipped.
func TestCommunityContract_Achievements(t *testing.T) {
	g := communityContractGame(t)
	body := communityContractRequest(t, communityHandler(), g, "/community/achievement/getAvailableAchievements")
	defs, ok := body["achievementDefs"].([]any)
	if !ok || len(defs) != 1 {
		t.Fatalf("achievementDefs = %v, want 1 (the deleted one is skipped)", body["achievementDefs"])
	}
	def, _ := defs[0].(map[string]any)
	assertKeys(t, "achievementDefs[0]", def,
		"id", "name", "name_locstringid", "description_locstringid",
		"icon", "timemodified", "deleted", "itembundle_id")
}

// TestCommunityContract_LeaderboardStat has 18 fields, including the three-value
// ranklevel (§3.3, C13).
func TestCommunityContract_LeaderboardStat(t *testing.T) {
	stat := wireFields(t, shared.NewStat(200, nil))
	assertKeys(t, "leaderboardStats[0]", stat,
		"statgroup_id", "leaderboard_id", "wins", "losses", "streak", "disputes", "drops",
		"rank", "ranktotal", "ranklevel", "rating", "regionrank", "regionranktotal",
		"lastmatchdate", "highestrank", "highestranklevel", "highestrating", "metadata")
}

// TestCommunityContract_MatchHistory checks the match objects and the
// civilization_id/race_id rename.
func TestCommunityContract_MatchHistory(t *testing.T) {
	g := communityContractGame(t)
	store := g.MatchHistory()
	store.RecordMatch(models.MatchResult{
		MatchID:       4242,
		MapName:       "Arabia",
		MaxPlayers:    2,
		LeaderboardID: 3,
		StartedAt:     time.Unix(1700000000, 0),
		FinishedAt:    time.Unix(1700000500, 0),
		// ApplyResult only persists a match that names its players and teams,
		// so both the flat arrays and the typed rows are provided.
		Players:  []int32{111, 222},
		Teams:    []int32{1, 2},
		Results:  []int32{1, 0},
		Races:    []int32{7, 8},
		XPGained: []int32{10, 5},
		Counters: []string{`{"a":1}`, ""},
		PlayerStats: []models.MatchPlayer{
			{ProfileID: 111, TeamID: 1, Result: 1, RaceID: 7, XPGained: 10, Counters: `{"a":1}`},
			{ProfileID: 222, TeamID: 2, Result: 0, RaceID: 8, XPGained: 5},
		},
	})

	body := communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/getRecentMatchHistory?profile_ids="+url.QueryEscape("[111]"))
	matches, ok := body["matchHistoryStats"].([]any)
	if !ok || len(matches) != 1 {
		t.Fatalf("matchHistoryStats = %v, want 1 match", body["matchHistoryStats"])
	}
	match, _ := matches[0].(map[string]any)
	assertKeys(t, "match", match,
		"id", "creator_profile_id", "mapname", "maxplayers", "matchtype_id", "options",
		"slotinfo", "description", "startgametime", "completiontime", "observertotal",
		"matchhistoryreportresults", "matchhistoryitems", "matchurls", "matchhistorymember",
		"gamemod_id")

	reportResults, _ := match["matchhistoryreportresults"].([]any)
	if len(reportResults) != 2 {
		t.Fatalf("matchhistoryreportresults = %v, want 2", match["matchhistoryreportresults"])
	}
	report, _ := reportResults[0].(map[string]any)
	assertKeys(t, "reportresults[0]", report,
		"matchhistory_id", "profile_id", "resulttype", "teamid", "civilization_id",
		"xpgained", "counters", "matchstartdate")
	if _, present := report["race_id"]; present {
		t.Error("reportresults[0] still carries race_id; the API renamed it to civilization_id")
	}

	members, _ := match["matchhistorymember"].([]any)
	if len(members) != 2 {
		t.Fatalf("matchhistorymember = %v, want 2", match["matchhistorymember"])
	}
	member, _ := members[0].(map[string]any)
	assertKeys(t, "matchhistorymember[0]", member,
		"matchhistory_id", "profile_id", "civilization_id", "statgroup_id", "teamid",
		"wins", "losses", "streak", "arbitration", "outcome", "oldrating", "newrating",
		"reporttype")
	if _, present := member["race_id"]; present {
		t.Error("matchhistorymember[0] still carries race_id")
	}

	profiles, ok := body["profiles"].([]any)
	if !ok || len(profiles) == 0 {
		t.Fatalf("profiles = %v, want at least one", body["profiles"])
	}
	profile, _ := profiles[0].(map[string]any)
	assertKeys(t, "profiles[0]", profile,
		"profile_id", "name", "alias", "personal_statgroup_id", "xp", "level",
		"leaderboardregion_id", "country", "clanlist_name")
}

// TestCommunityContract_MatchHistoryKey checks that the optional endpoint uses
// matchHistory as its key, not matchHistoryStats (C5).
func TestCommunityContract_MatchHistoryKey(t *testing.T) {
	g := communityContractGame(t)
	g.MatchHistory().RecordMatch(models.MatchResult{
		MatchID:    5353,
		MapName:    "Arena",
		StartedAt:  time.Unix(1700000000, 0),
		FinishedAt: time.Unix(1700000500, 0),
		Players:    []int32{333},
		Teams:      []int32{1},
		Results:    []int32{1},
		Races:      []int32{2},
	})
	body := communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/getMatchHistory?matchIDs="+url.QueryEscape("[5353]"))
	if _, present := body["matchHistory"]; !present {
		t.Fatalf("getMatchHistory: missing matchHistory key, got %v", sortedKeys(body))
	}
	matches, _ := body["matchHistory"].([]any)
	if len(matches) != 1 {
		t.Fatalf("matchHistory = %v, want 1", body["matchHistory"])
	}
	match, _ := matches[0].(map[string]any)
	// getMatchHistory reports the real game mode; getRecentMatchHistory sends 0.
	if gamemod, _ := match["gamemod_id"].(float64); int(gamemod) != 1800 {
		t.Fatalf("gamemod_id = %v, want 1800", match["gamemod_id"])
	}
}

// TestCommunityContract_ReplayFiles checks the shared matchurl/replayfile shape.
func TestCommunityContract_ReplayFiles(t *testing.T) {
	g := communityContractGame(t)
	body := communityContractRequest(t, communityHandler(), g, "/community/leaderboard/getReplayFiles?matchIDs="+url.QueryEscape("[1]"))
	if _, present := body["expiryUnix"]; !present {
		t.Fatal("getReplayFiles: missing expiryUnix")
	}
	if files, ok := body["replayFiles"].([]any); !ok || len(files) != 0 {
		t.Fatalf("replayFiles = %v, want an empty array", body["replayFiles"])
	}
}

// TestCommunityContract_FindAdvertisements injects a lobby and verifies the
// object encoding, including the security-critical absences: no password value,
// no observer password and no internal IP.
func TestCommunityContract_FindAdvertisements(t *testing.T) {
	g := communityContractGame(t)
	host := g.Users().GetOrCreateUser(game.AoE2, g.Items(), nil, "127.0.0.1:1", "00:11:22:33:44:55", false, 76561198000000009, "host")
	lobby := g.Advertisements().Store(communityHostRequest(), true, game.AoE2)
	if lobby == nil {
		t.Fatal("could not create the advertisement")
	}
	g.Advertisements().WithWriteLock(lobby.GetId(), func() {
		lobby.UnsafeSetHostId(host.GetId())
	})

	body := communityContractRequest(t, communityHandler(), g, "/community/advertisement/findAdvertisements")
	matches, ok := body["matches"].([]any)
	if !ok || len(matches) != 1 {
		t.Fatalf("matches = %v, want 1", body["matches"])
	}
	match, _ := matches[0].(map[string]any)
	assertKeys(t, "matches[0]", match,
		"id", "steamlobbyid", "goodoldgameslobbyid", "xboxsessionid", "host_profile_id",
		"state", "description", "visible", "mapname", "options", "passwordprotected",
		"maxplayers", "slotinfo", "matchtype_id", "matchmembers", "observernum",
		"observermax", "isobservable", "observerdelay", "hasobserverpassword",
		"servicetype", "relayserver_region")

	encoded, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	serialized := strings.ToLower(string(encoded))
	// The secrets themselves must never appear: only their boolean presence
	// flags may. The internal advertisement IP is dropped too.
	for _, secret := range []string{"lobby-secret", "observer-secret", "10.0.11."} {
		if strings.Contains(serialized, strings.ToLower(secret)) {
			t.Errorf("lobby JSON leaks %q: %s", secret, serialized)
		}
	}
	// The password-bearing members are reported as presence flags. They are 0/1
	// numbers, not JSON booleans: that is what the live service sends and what
	// the game protocol uses for every other flag too.
	if protected, _ := match["passwordprotected"].(float64); int(protected) != 1 {
		t.Errorf("passwordprotected = %v, want 1", match["passwordprotected"])
	}
	if hasObserverPassword, _ := match["hasobserverpassword"].(float64); int(hasObserverPassword) != 1 {
		t.Errorf("hasobserverpassword = %v, want 1", match["hasobserverpassword"])
	}
	if visible, _ := match["visible"].(float64); int(visible) != 1 {
		t.Errorf("visible = %v, want 1", match["visible"])
	}
	members, _ := match["matchmembers"].([]any)
	for _, raw := range members {
		member, _ := raw.(map[string]any)
		if _, present := member["ip"]; present {
			t.Error("matchmembers[] exposes the internal ip")
		}
	}
	if _, present := match["avatars"]; present {
		t.Error("avatars must be a sibling of matches, not inside it")
	}
	if avatars, ok := body["avatars"].([]any); !ok {
		t.Errorf("avatars = %v, want an array", body["avatars"])
	} else if len(avatars) != 0 {
		// The host was set after storing; members only exist once a peer joins.
		t.Logf("avatars = %v", avatars)
	}
}

// TestCommunityContract_PersonalStatAcrossLadders checks that GetPersonalStat
// returns one row per ladder, with the real rank filled (§2.b, C4).
func TestCommunityContract_PersonalStatAcrossLadders(t *testing.T) {
	g := communityContractGame(t)
	store := g.MatchHistory()
	// Two matches on different ladders for the same profile.
	for index, leaderboard := range []int32{3, 4} {
		store.RecordMatch(models.MatchResult{
			MatchID:       int64(7000 + index),
			LeaderboardID: leaderboard,
			FinishedAt:    time.Unix(int64(1700000000+index), 0),
			Players:       []int32{4242, int32(5000 + index)},
			Teams:         []int32{1, 2},
			Results:       []int32{1, 0},
			Races:         []int32{1, 2},
		})
	}
	body := communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/GetPersonalStat?profile_ids="+url.QueryEscape("[4242]"))
	stats, ok := body["leaderboardStats"].([]any)
	if !ok || len(stats) != 2 {
		t.Fatalf("leaderboardStats = %v, want 2 rows (one per ladder)", body["leaderboardStats"])
	}
	seen := make(map[float64]bool, len(stats))
	for _, raw := range stats {
		row, _ := raw.(map[string]any)
		if leaderboard, _ := row["leaderboard_id"].(float64); leaderboard > 0 {
			seen[leaderboard] = true
		}
		// A profile with a rating row is ranked, so ranklevel must not be 0.
		if rankLevel, _ := row["ranklevel"].(float64); int(rankLevel) != 1 {
			t.Errorf("ranklevel = %v, want 1 for a rated profile", row["ranklevel"])
		}
	}
	if !seen[3] || !seen[4] {
		t.Fatalf("ladders = %v, want 3 and 4", seen)
	}
}

// TestCommunityContract_ConnectedUserProfileAndAvatarStats checks the encoder
// branch that only a connected user reaches: profileFromUser and the avatar stat
// rows. An offline profile has neither.
func TestCommunityContract_ConnectedUserProfileAndAvatarStats(t *testing.T) {
	g := communityContractGame(t)
	user := g.Users().GetOrCreateUser(game.AoE2, g.Items(), g.LeaderboardDefinitions().AvatarStatDefinitions(),
		"127.0.0.1:9999", "00:11:22:33:44:55", false, 76561197984749679, "Oni.TheViper")
	if user == nil {
		t.Fatal("could not create the user")
	}
	// The fake leaderboards resource carries no avatar-stat definitions, so the
	// user starts with an empty set. Adding one exercises the encoder without
	// depending on the bundled catalogue.
	avatarStats := user.GetAvatarStats()
	if avatarStats == nil {
		t.Fatal("the user has no avatar stats document")
	}
	if err := avatarStats.WithReadWrite(func(data *models.AvatarStats) error {
		data.AddStat(models.NewAvatarStat(1, 3215))
		return nil
	}); err != nil {
		t.Fatalf("seed avatar stat: %v", err)
	}

	// The profile object must carry the live identity.
	body := communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/GetPersonalStat?profile_ids="+url.QueryEscape("["+itoa(user.GetId())+"]"))
	groups, _ := body["statGroups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("statGroups = %v, want 1", body["statGroups"])
	}
	group, _ := groups[0].(map[string]any)
	members, _ := group["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("members = %v, want 1", group["members"])
	}
	member, _ := members[0].(map[string]any)
	if name, _ := member["name"].(string); name != "/steam/76561197984749679" {
		t.Errorf("member name = %v, want the platform path", member["name"])
	}
	if alias, _ := member["alias"].(string); alias != "Oni.TheViper" {
		t.Errorf("member alias = %v, want the live alias", member["alias"])
	}

	// And the avatar stats exist because the user is connected.
	body = communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/GetAvatarStatForProfile?profile_names="+
			url.QueryEscape(`["/steam/76561197984749679"]`))
	stats, ok := body["avatarStatsForProfile"].([]any)
	if !ok {
		t.Fatalf("avatarStatsForProfile = %v, want an array", body["avatarStatsForProfile"])
	}
	if len(stats) == 0 {
		t.Fatal("avatarStatsForProfile is empty for a connected user")
	}
	stat, _ := stats[0].(map[string]any)
	assertKeys(t, "avatarStatsForProfile[0]", stat,
		"profile_id", "avatarstat_id", "value", "lastupdated")
}

// TestCommunityContract_ReplayFilesWithStoredReplay checks the replay URL encoder
// and that expiryUnix is reported when replay storage is enabled.
func TestCommunityContract_ReplayFilesWithStoredReplay(t *testing.T) {
	g := communityContractGame(t)
	cloudFiles := g.Resources().CloudFiles()
	if cloudFiles == nil {
		t.Fatal("no cloud store")
	}
	const key = "replays/424242/0/replay.gz"
	if _, err := cloudFiles.Put(key, func(writer io.Writer) error {
		_, writeErr := writer.Write([]byte("replay-bytes"))
		return writeErr
	}); err != nil {
		t.Fatalf("store replay: %v", err)
	}
	// ReplaysExpiry and ReplaysEnabled are process-global; set both so expiryUnix
	// is a real timestamp.
	previousExpiry, previousEnabled := i.ReplaysExpiry, i.ReplaysEnabled
	i.ReplaysExpiry = time.Hour
	i.ReplaysEnabled = true
	t.Cleanup(func() {
		i.ReplaysExpiry = previousExpiry
		i.ReplaysEnabled = previousEnabled
	})

	body := communityContractRequest(t, communityHandler(), g,
		"/community/leaderboard/getReplayFiles?matchIDs="+url.QueryEscape("[424242]"))
	files, ok := body["replayFiles"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("replayFiles = %v, want 1", body["replayFiles"])
	}
	file, _ := files[0].(map[string]any)
	assertKeys(t, "replayFiles[0]", file, "profile_id", "matchhistory_id", "url", "size", "datatype")
	if fileURL, _ := file["url"].(string); !strings.Contains(fileURL, "/cloudfiles/"+key) {
		t.Errorf("url = %v, want it to point at /cloudfiles/%s", file["url"], key)
	}
	if expiry, _ := body["expiryUnix"].(float64); expiry <= 0 {
		t.Errorf("expiryUnix = %v, want a future timestamp", body["expiryUnix"])
	}
}

// TestCommunityContract_ReplayFilesDisabled checks the disabled branch: no files
// and expiryUnix 0.
func TestCommunityContract_ReplayFilesDisabled(t *testing.T) {
	g := communityContractGame(t)
	g.Resources().CloudFiles().SetReplayOpts(models.CloudFilesReplayOpts{Enabled: false})
	body := communityContractRequest(t, communityHandler(), g, "/community/leaderboard/getReplayFiles?matchIDs="+url.QueryEscape("[1]"))
	if files, ok := body["replayFiles"].([]any); !ok || len(files) != 0 {
		t.Fatalf("replayFiles = %v, want empty", body["replayFiles"])
	}
	if expiry, _ := body["expiryUnix"].(float64); expiry != 0 {
		t.Fatalf("expiryUnix = %v, want 0 when replays are disabled", body["expiryUnix"])
	}
}

// itoa keeps the test free of strconv noise at the call site.
func itoa(value int32) string {
	return strconv.FormatInt(int64(value), 10)
}

// communityHostRequest builds a waiting, visible lobby with a password and an
// observer password set, so the absence checks are meaningful.
func communityHostRequest() *advertisementShared.AdvertisementHostRequest {
	return &advertisementShared.AdvertisementHostRequest{
		Race:             3,
		Team:             1,
		Party:            -1,
		VersionFlags:     1,
		Description:      "community test lobby",
		MapName:          "Arabia",
		Observable:       true,
		ObserverDelay:    120,
		ObserverPassword: "observer-secret",
		Password:         "lobby-secret",
		Passworded:       true,
		Visible:          true,
		Joinable:         true,
		MatchType:        6,
		MaxPlayers:       4,
		Options:          "b3B0aW9ucw==",
		SlotInfo:         "c2xvdHM=",
		State:            0,
		RelayRegion:      "westeurope",
	}
}
