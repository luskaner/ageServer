package party

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// The success code is part of the wire protocol with the client: it must be a
// single value, not a range.
func TestReportMatchResponseSuccessCode(t *testing.T) {
	response := reportMatchResponse(0, nil)
	if len(response) != 16 {
		t.Fatalf("response has %d fields, want 16", len(response))
	}
	if response[0] != 0 {
		t.Fatalf("success code = %v, want 0 (verified against client 1.16.0)", response[0])
	}
}

func TestReportMatchRequestBindsClientReportFields(t *testing.T) {
	form := url.Values{
		"match_id":    {"42"},
		"profile_ids": {"[1,2]"},
		"results":     {"[1,0]"},
		"teamIDs":     {"[0,1]"},
		"race_ids":    {"[4,5]"},
		"xpGained":    {"[1,1]"},
		"countersZip": {`"H4sI"`},
		"itemUpdates": {`["{\"itemUpdates\":[],\"challengeUpdates\":[],\"clientUpdates\":{}}","{\"itemUpdates\":[],\"challengeUpdates\":[],\"clientUpdates\":{}}"]`},
	}
	request := httptest.NewRequest("POST", "/game/party/reportMatch", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var got reportMatchRequest
	if err := i.Bind(request, &got); err != nil {
		t.Fatal(err)
	}
	if got.MatchID != 42 || len(got.ProfileIDs.Data) != 2 || got.CountersZip != `"H4sI"` {
		t.Fatalf("bound request = %+v", got)
	}
	var parsed i.Json[[]string]
	if err := parsed.UnmarshalText([]byte(got.ItemUpdates)); err != nil {
		t.Fatalf("itemUpdates did not parse: %v", err)
	}
	if len(parsed.Data) != 2 {
		t.Fatalf("itemUpdates = %v", parsed.Data)
	}
}

// A malformed itemUpdates must not fail the bind nor drop the match: the
// handler logs it and falls back to "no update", which preserves what is
// stored.
func TestReportMatchToleratesMalformedItemUpdates(t *testing.T) {
	form := url.Values{
		"match_id":    {"43"},
		"profile_ids": {"[1]"},
		"results":     {"[1]"},
		"teamIDs":     {"[0]"},
		"race_ids":    {"[4]"},
		"xpGained":    {"[1]"},
		"itemUpdates": {"not-json"},
	}
	request := httptest.NewRequest("POST", "/game/party/reportMatch", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var got reportMatchRequest
	if err := i.Bind(request, &got); err != nil {
		t.Fatalf("bind must tolerate a malformed itemUpdates: %v", err)
	}
	var parsed i.Json[[]string]
	if err := parsed.UnmarshalText([]byte(got.ItemUpdates)); err == nil {
		t.Fatal("expected a parse error for the malformed itemUpdates")
	}
	updates, err := normalizeMatchCustomUpdates(parsed.Data, 1)
	if err != nil {
		t.Fatalf("normalizeMatchCustomUpdates: %v", err)
	}
	if len(updates) != 1 || updates[0] != "" {
		t.Fatalf("custom updates = %#v, want one empty value (keep stored)", updates)
	}
}

func TestDecodeReportCounters(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`["{\"villagers\":12}","{}"]`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(compressed.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	got, err := decodeReportCounters(string(encoded), 2)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != `{"villagers":12}` || got[1] != "{}" {
		t.Fatalf("decoded counters = %#v", got)
	}
}

func TestDecodeReportCountersReturnsEmptyWhenAbsent(t *testing.T) {
	got, err := decodeReportCounters("", 2)
	if err != nil {
		t.Fatalf("decodeReportCounters: %v", err)
	}
	if len(got) != 2 || got[0] != "" || got[1] != "" {
		t.Fatalf("counters = %#v, want two empty values so the stored ones are kept", got)
	}
}

func TestDecodeReportCountersReturnsEmptyOnMalformed(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`["{}"]`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(compressed.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	// Wrong player count: the error is reported but the values stay empty so
	// the stored counters are preserved.
	got, err := decodeReportCounters(string(encoded), 2)
	if err == nil {
		t.Fatal("expected an error for a counter count mismatch")
	}
	if len(got) != 2 || got[0] != "" || got[1] != "" {
		t.Fatalf("counters = %#v, want empty values (not \"{}\")", got)
	}
}

func TestNormalizeMatchCustomUpdatesPreservesUnknownKeys(t *testing.T) {
	got, err := normalizeMatchCustomUpdates([]string{
		`{"itemUpdates":["item"],"challengeUpdates":[],"clientUpdates":{},"customScoring":{"points":2}}`,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"challengeUpdates":[],"clientUpdates":{},"customScoring":{"points":2},"eventChallengeUpdates":[],"itemUpdates":["item"]}`
	if got[0] != want {
		t.Fatalf("normalized custom updates = %s, want %s", got[0], want)
	}
}

func TestNormalizeMatchCustomUpdatesReturnsEmptyWhenAbsent(t *testing.T) {
	got, err := normalizeMatchCustomUpdates(nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "" || got[1] != "" {
		t.Fatalf("custom updates = %#v, want empty values so the stored ones are kept", got)
	}
}

func TestCountersPairsKeepsClientOrder(t *testing.T) {
	pairs := countersPairs(`{"z":1,"a":2,"m":3}`)
	if len(pairs) != 3 {
		t.Fatalf("pairs = %#v", pairs)
	}
	for idx, want := range []string{"z", "a", "m"} {
		pair, ok := pairs[idx].([]any)
		if !ok || pair[0] != want {
			t.Fatalf("pair %d = %#v, want key %q", idx, pairs[idx], want)
		}
	}
}

func TestCountersPairsToleratesMissingOrMalformed(t *testing.T) {
	for _, raw := range []string{"", "   ", "not json", "[]", `{"a":`} {
		if pairs := countersPairs(raw); len(pairs) != 0 {
			t.Errorf("countersPairs(%q) = %#v, want empty", raw, pairs)
		}
	}
}

func TestGameResultNotificationMatchesOfficialShape(t *testing.T) {
	match := models.MatchResult{
		MatchID:   518002788,
		QueueType: "automatch2-6",
		Players:   []int32{24916071, 25344413},
		Teams:     []int32{0, 1},
		Results:   []int32{1, 0},
		Races:     []int32{199703, 129267},
		XPGained:  []int32{4, 2},
		Counters:  []string{`{"unitprod":0,"totalcmds":87}`, `{"unitprod":1,"totalcmds":12}`},
	}
	ratings := map[int32]*models.PlayerRating{
		24916071: {
			ProfileID:     24916071,
			LeaderboardID: models.SoloRMRanked,
			Rating:        732,
			HighestRating: 860,
			Wins:          369,
			Losses:        369,
			Streak:        2,
			LastMatch:     time.Unix(1793552203, 0),
		},
		25344413: {
			ProfileID:     25344413,
			LeaderboardID: models.SoloRMRanked,
			Rating:        701,
			HighestRating: 754,
			Wins:          24,
			Losses:        31,
			Streak:        -2,
			LastMatch:     time.Unix(1793552203, 0),
		},
	}

	got := gameResultNotification("age2", match, ratings, map[int32]int32{
		24916071: 13436792,
		25344413: 13657770,
	})
	if len(got) != 6 || got[3] != int64(518002788) {
		t.Fatalf("notification envelope = %#v", got)
	}
	playerResults, ok := got[0].([]any)
	if !ok || len(playerResults) != 2 {
		t.Fatalf("player results = %#v", got[0])
	}
	first, ok := playerResults[0].([]any)
	if !ok || len(first) != 17 {
		t.Fatalf("player result row = %#v", playerResults[0])
	}
	if first[0] != int32(24916071) || first[1] != int32(6) || first[2] != int32(199703) || first[3] != int32(0) || first[5] != int32(13436792) {
		t.Fatalf("player result fields = %#v", first)
	}
	// Slot 4 is the list of winning teams, shared by every row. The client looks
	// its own team up in it: a fixed [0] made every match a victory and the
	// client answered with a desync dialog.
	winning, ok := first[4].([]any)
	if !ok || len(winning) != 1 || winning[0] != int32(0) {
		t.Fatalf("winning teams = %#v", first[4])
	}
	second, ok := playerResults[1].([]any)
	if !ok || second[3] != int32(1) || second[4].([]any)[0] != int32(0) {
		t.Fatalf("second player result = %#v", playerResults[1])
	}
	// The official row carries the race, the counters as [[key, value], ...] in
	// the order the client sent them, and the XP gained.
	counters, ok := first[6].([]any)
	if !ok || len(counters) != 2 {
		t.Fatalf("counters = %#v", first[6])
	}
	if pair, ok := counters[0].([]any); !ok || pair[0] != "unitprod" || pair[1] != json.Number("0") {
		t.Fatalf("first counter pair = %#v", counters[0])
	}
	if pair, ok := counters[1].([]any); !ok || pair[0] != "totalcmds" || pair[1] != json.Number("87") {
		t.Fatalf("second counter pair = %#v", counters[1])
	}
	if first[8] != int32(4) {
		t.Fatalf("xpGained = %#v", first[8])
	}
	stats, ok := got[1].([]any)
	if !ok || len(stats) != 2 {
		t.Fatalf("leaderboard stats = %#v", got[1])
	}
	stat, ok := stats[0].([]any)
	if !ok || len(stat) != 19 || stat[0] != int32(13436792) || stat[1] != int32(models.SoloRMRanked) || stat[12] != int32(732) {
		t.Fatalf("leaderboard stat row = %#v", stats[0])
	}
}

// A match the local player lost has no winning team, so slot 4 must stay empty.
// The official service sends [] there and the client then reads the match as a
// defeat; with a populated slot 4 it reads a victory and reports a desync.
func TestGameResultNotificationHasNoWinningTeamOnDefeat(t *testing.T) {
	match := models.MatchResult{
		MatchID: 908104717,
		Players: []int32{1786186287},
		Teams:   []int32{0},
		Results: []int32{0},
		Races:   []int32{9005920},
	}
	got := gameResultNotification("age2", match, nil, map[int32]int32{1786186287: 195658309})
	rows, ok := got[0].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("player results = %#v", got[0])
	}
	row, ok := rows[0].([]any)
	if !ok || len(row) != 17 {
		t.Fatalf("player result row = %#v", rows[0])
	}
	if row[3] != int32(0) {
		t.Fatalf("team = %#v", row[3])
	}
	if winning, ok := row[4].([]any); !ok || len(winning) != 0 {
		t.Fatalf("winning teams = %#v, want []", row[4])
	}
}

// AoE III sends simplayerIDs=[4294967295] (0xFFFFFFFF, "no sim player"). With
// int32 the whole bind failed and the match was dropped, so the report never
// reached the history and no replay descriptor was issued.
func TestReportMatchBindsUnsignedSimPlayerIDs(t *testing.T) {
	form := url.Values{
		"match_id":     {"1426698940"},
		"profile_ids":  {"[1786186287]"},
		"results":      {"[3]"},
		"teamIDs":      {"[1]"},
		"race_ids":     {"[16]"},
		"xpGained":     {"[0]"},
		"simplayerIDs": {"[4294967295]"},
		"checkSums":    {"[0]"},
	}
	request := httptest.NewRequest("POST", "/game/party/reportMatch", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var got reportMatchRequest
	if err := i.Bind(request, &got); err != nil {
		t.Fatalf("bind must accept an unsigned simplayerIDs: %v", err)
	}
	if len(got.SimPlayerIDs.Data) != 1 || got.SimPlayerIDs.Data[0] != 4294967295 {
		t.Fatalf("simplayerIDs = %v, want [4294967295]", got.SimPlayerIDs.Data)
	}
}
