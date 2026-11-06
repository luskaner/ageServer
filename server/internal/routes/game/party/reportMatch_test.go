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

func TestGameResultNotificationMatchesOfficialShape(t *testing.T) {
	match := models.MatchResult{
		MatchID: 518002788,
		Players: []int32{24916071, 25344413},
		Results: []int32{1, 0},
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

	got := gameResultNotification(match, ratings, map[int32]int32{
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
	if first[0] != int32(24916071) || first[1] != int32(6) || first[2] != int32(4) || first[3] != int32(1) || first[5] != int32(13436792) {
		t.Fatalf("player result fields = %#v", first)
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
