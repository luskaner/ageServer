package leaderboard

import (
	"testing"
	"time"

	"github.com/luskaner/ageServer/server/internal/models"
)

func TestMatchHistoryReportResultUsesStoredPlayerData(t *testing.T) {
	match := models.MatchResult{
		MatchID:   42,
		StartedAt: time.Unix(1_700_000_000, 0),
	}
	player := models.MatchPlayer{
		ProfileID:     123,
		TeamID:        1,
		Result:        1,
		RaceID:        24,
		XPGained:      1,
		Counters:      `{"villagers":12}`,
		CustomUpdates: `{"itemUpdates":[],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`,
	}

	got := matchHistoryReportResult(match, player)
	if len(got) != 9 {
		t.Fatalf("report result has %d fields, want 9", len(got))
	}
	want := []any{
		int64(42), int32(123), int32(1), int32(1), int32(24), int32(1),
		`{"villagers":12}`,
		`{"itemUpdates":[],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`,
		int64(1_700_000_000),
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Errorf("field %d = %#v, want %#v", idx, got[idx], want[idx])
		}
	}
}

func TestMatchHistoryReportResultDefaultsMissingClientDetails(t *testing.T) {
	got := matchHistoryReportResult(models.MatchResult{}, models.MatchPlayer{})
	if got[6] != "{}" {
		t.Errorf("counters = %v, want {}", got[6])
	}
	if got[7] != `{"itemUpdates":[],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}` {
		t.Errorf("custom updates = %v", got[7])
	}
}
