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

func TestReportMatchResponseResultCodes(t *testing.T) {
	for _, resultCode := range []int{0, 2} {
		response := reportMatchResponse(resultCode)
		if len(response) != 16 {
			t.Fatalf("response has %d fields, want 16", len(response))
		}
		if got := response[0]; got != resultCode {
			t.Errorf("result code = %v, want %d", got, resultCode)
		}
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
	if got.MatchID != 42 || len(got.ProfileIDs.Data) != 2 || len(got.ItemUpdates.Data) != 2 || got.CountersZip != `"H4sI"` {
		t.Fatalf("bound request = %+v", got)
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

func TestNormalizeMatchCustomUpdatesMatchesHistoryProtocol(t *testing.T) {
	got, err := normalizeMatchCustomUpdates([]string{
		`{"itemUpdates":["item"],"challengeUpdates":[],"clientUpdates":{},"customScoring":{"points":2}}`,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"itemUpdates":["item"],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`
	if got[0] != want {
		t.Fatalf("normalized custom updates = %s, want %s", got[0], want)
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
