package router

import (
	"encoding/json"
	"strings"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// ratingWith builds the ladder row used by the encoder checks.
func ratingWith(rankTotal, rank int64) *models.PlayerRating {
	return &models.PlayerRating{
		ProfileID:     111,
		LeaderboardID: 3,
		StatID:        200,
		Rating:        2044,
		HighestRating: 2100,
		Wins:          10,
		Losses:        5,
		Streak:        2,
		Rank:          rank,
		RankTotal:     rankTotal,
	}
}

// wireFields marshals a response struct and returns its JSON object, which is
// the contract that actually reaches a client. Checking the struct fields
// directly would not prove the tags are right.
func wireFields(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("cannot marshal %T: %v", value, err)
	}
	var fields map[string]any
	if err = json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("cannot unmarshal %T: %v (body=%s)", value, err, encoded)
	}
	return fields
}

// TestCommunityStat_RankLevelCoversTheThreeValues checks the ranklevel tri-state
// (C13, matiz): -1 when the ladder population is unknown, 0 with a row but no
// rank, 1 with a rank.
func TestCommunityStat_RankLevelCoversTheThreeValues(t *testing.T) {
	cases := []struct {
		name      string
		rankTotal int64
		rank      int64
		want      int32
	}{
		{name: "unknown population", rankTotal: -1, rank: -1, want: -1},
		{name: "empty ladder", rankTotal: 0, rank: 0, want: 0},
		{name: "ranked", rankTotal: 47831, rank: 13, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stat := shared.NewStat(200, ratingWith(tc.rankTotal, tc.rank))
			if stat.RankLevel != tc.want {
				t.Fatalf("ranklevel = %d, want %d", stat.RankLevel, tc.want)
			}
		})
	}
}

// TestCommunityStat_RankTotalKeepsSentinels checks that -1, 0 and a positive
// population all reach the response unchanged.
func TestCommunityStat_RankTotalKeepsSentinels(t *testing.T) {
	for _, want := range []int64{-1, 0, 47831} {
		stat := shared.NewStat(200, ratingWith(want, 0))
		if stat.RankTotal != want {
			t.Fatalf("ranktotal = %d, want %d", stat.RankTotal, want)
		}
	}
}

// TestCommunityStat_HasEighteenFields pins the wire contract: the wiki documents
// 14 fields and the live service returns 18 (C13).
func TestCommunityStat_HasEighteenFields(t *testing.T) {
	fields := wireFields(t, shared.NewStat(7, nil))
	want := []string{
		"statgroup_id", "leaderboard_id", "wins", "losses", "streak", "disputes",
		"drops", "rank", "ranktotal", "ranklevel", "rating", "regionrank",
		"regionranktotal", "lastmatchdate", "highestrank", "highestranklevel",
		"highestrating", "metadata",
	}
	if len(fields) != len(want) {
		t.Fatalf("nil rating produced %d fields, want %d: %v", len(fields), len(want), sortedKeys(fields))
	}
	for _, key := range want {
		if _, present := fields[key]; !present {
			t.Errorf("stat is missing %q", key)
		}
	}
}

// TestCommunityStat_EmptyRatingIsStillComplete checks a nil rating row: the
// endpoints emit one for a profile that has never played, and the fields must be
// the documented sentinels rather than zero values that mean something else.
func TestCommunityStat_EmptyRatingIsStillComplete(t *testing.T) {
	stat := shared.NewStat(7, nil)
	if stat.Rating != models.DefaultRating {
		t.Fatalf("rating = %d, want the default %d", stat.Rating, models.DefaultRating)
	}
	if stat.Rank != -1 {
		t.Fatalf("rank = %d, want -1", stat.Rank)
	}
	if stat.LastMatchDate != 0 {
		t.Fatalf("lastmatchdate = %d, want 0", stat.LastMatchDate)
	}
	if stat.RegionRank != -1 {
		t.Fatalf("regionrank = %d, want -1", stat.RegionRank)
	}
}

// TestCommunityProfile_HasNineFields checks the profile object shape: nine named
// fields, including the two this server cannot fill (C14).
func TestCommunityProfile_HasNineFields(t *testing.T) {
	want := []string{
		"profile_id", "name", "alias", "personal_statgroup_id", "xp", "level",
		"leaderboardregion_id", "country", "clanlist_name",
	}
	fields := wireFields(t, shared.Profile{})
	if len(fields) != len(want) {
		t.Fatalf("empty profile has %d fields, want %d: %v", len(fields), len(want), sortedKeys(fields))
	}
	for _, key := range want {
		if _, present := fields[key]; !present {
			t.Errorf("profile is missing %q", key)
		}
	}
	// The live user handle must never reach the wire.
	live := shared.Profile{ProfileID: 5, User: nil}
	if _, present := wireFields(t, live)["User"]; present {
		t.Error("profile leaks the User field onto the wire")
	}
}

// TestCommunityMatchURLShapes checks the two artifact types: matchurls[] has no
// match id, replayFiles[] carries it (C15).
func TestCommunityMatchURLShapes(t *testing.T) {
	url := wireFields(t, shared.MatchURL{ProfileID: 7, URL: "u", Size: 10})
	if _, present := url["matchhistory_id"]; present {
		t.Error("matchurls[] must not carry matchhistory_id")
	}
	for _, key := range []string{"profile_id", "url", "size", "datatype"} {
		if _, present := url[key]; !present {
			t.Errorf("matchurls[] is missing %q", key)
		}
	}
	replay := wireFields(t, shared.ReplayFile{ProfileID: 7, MatchHistoryID: 42, URL: "u", Size: 1})
	if id, _ := replay["matchhistory_id"].(float64); int64(id) != 42 {
		t.Errorf("replayFiles[] matchhistory_id = %v, want 42", replay["matchhistory_id"])
	}
}

// TestCommunityResolveStatGroupID checks the join key: the stored stat group
// wins over the rating's, and the profile id is the last resort.
func TestCommunityResolveStatGroupID(t *testing.T) {
	profile := &shared.Profile{ProfileID: 500, StatGroupID: 200}
	rating := ratingWith(10, 1)
	rating.StatID = 900
	if got := shared.StatGroupIDOf(profile, rating); got != 200 {
		t.Fatalf("stat group = %d, want the stored 200", got)
	}
	// Without a stored stat group the rating's stat id is used.
	if got := shared.StatGroupIDOf(&shared.Profile{ProfileID: 500}, rating); got != 900 {
		t.Fatalf("stat group = %d, want the rating stat id 900", got)
	}
	// With neither, the profile id keeps the join non-zero.
	if got := shared.StatGroupIDOf(&shared.Profile{ProfileID: 500}, nil); got != 500 {
		t.Fatalf("stat group = %d, want the profile id 500", got)
	}
}

// TestCommunityParsePlatformPath checks the only accepted spelling of
// profile_names, including the traversal-shaped inputs that must be rejected
// (§1.5, §7.4).
func TestCommunityParsePlatformPath(t *testing.T) {
	valid := map[string]struct {
		platformID uint64
		isXbox     bool
	}{
		"/steam/76561197984749679":   {76561197984749679, false},
		"/xboxlive/2533274812345678": {2533274812345678, true},
	}
	for value, want := range valid {
		got, ok := shared.ParsePlatformPath(value)
		if !ok {
			t.Errorf("%q was rejected", value)
			continue
		}
		if got.PlatformUserID != want.platformID || got.IsXbox != want.isXbox {
			t.Errorf("%q parsed as %+v, want %+v", value, got, want)
		}
	}
	invalid := []string{
		"/steam/../../../etc/passwd",
		"/steam/notanumber",
		"/psn/123",
		"/steam/",
		"/steam/0",
		"steam/123",
		"/steam/123/extra",
		"",
		"/steam/123\n",
	}
	for _, value := range invalid {
		if _, ok := shared.ParsePlatformPath(value); ok {
			t.Errorf("%q was accepted", value)
		}
	}
	// An over-long value is rejected before the pattern is even applied.
	if _, ok := shared.ParsePlatformPath("/steam/" + strings.Repeat("9", 200)); ok {
		t.Error("an over-long platform path was accepted")
	}
}

// TestCommunityProfileQueryNormalization checks the merge of the accepted
// spellings: placeholders dropped, duplicates removed, the cap enforced (§1.5).
func TestCommunityProfileQueryNormalization(t *testing.T) {
	query := shared.ProfileQuery{
		ProfileIDs:      i.Json[[]int32]{Data: []int32{3, 0, -1}},
		ProfileIDsNoSep: i.Json[[]int32]{Data: []int32{3, 4}},
		ProfileNames:    i.Json[[]string]{Data: []string{"/steam/1", " /steam/1 "}},
		Aliases:         i.Json[[]string]{Data: []string{"viper", "viper", ""}},
	}
	request := query.ProfileRequest()
	if len(request.ProfileIDs) != 2 || request.ProfileIDs[0] != 3 || request.ProfileIDs[1] != 4 {
		t.Fatalf("profile ids = %v, want [3 4] (placeholders dropped, duplicates removed)", request.ProfileIDs)
	}
	if len(request.Names) != 1 || request.Names[0] != "/steam/1" {
		t.Fatalf("profile names = %v, want [/steam/1] (duplicate dropped)", request.Names)
	}
	if len(request.Aliases) != 1 || request.Aliases[0] != "viper" {
		t.Fatalf("aliases = %v, want [viper] (empty and duplicate dropped)", request.Aliases)
	}
	// The cap holds for a caller that sends more than MaxQueryItems.
	long := make([]int32, shared.MaxQueryItems+10)
	for index := range long {
		long[index] = int32(index + 1)
	}
	capped := shared.ProfileQuery{ProfileIDs: i.Json[[]int32]{Data: long}}.ProfileRequest()
	if len(capped.ProfileIDs) != shared.MaxQueryItems {
		t.Fatalf("cap kept %d ids, want %d", len(capped.ProfileIDs), shared.MaxQueryItems)
	}
}

// TestCommunitySanitizeString checks the trimming and the length cap.
func TestCommunitySanitizeString(t *testing.T) {
	if got := shared.SanitizeString("  padded  "); got != "padded" {
		t.Fatalf("SanitizeString = %q, want the trimmed value", got)
	}
	if got := shared.SanitizeString(strings.Repeat("a", shared.MaxQueryString+1)); got != "" {
		t.Fatalf("SanitizeString kept an over-long value of %d bytes", len(got))
	}
	if got := shared.SanitizeString("ok"); got != "ok" {
		t.Fatalf("SanitizeString changed a valid value to %q", got)
	}
}
