package models

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *MatchHistoryStore {
	t.Helper()
	dir := t.TempDir()
	store := NewMatchHistoryStore(dir)
	if store.db == nil {
		t.Fatalf("database was not created in %s", dir)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestMatchHistoryCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	store := NewMatchHistoryStore(dir)
	if store.db == nil {
		t.Fatal("expected a usable database")
	}
	_ = store.Close()
	if _, err := os.Stat(filepath.Join(dir, "automatch2.db")); err != nil {
		t.Fatalf("database file not created: %s", err)
	}
}

func TestMatchHistoryCreatesSchemaAndReopens(t *testing.T) {
	dir := t.TempDir()
	store := NewMatchHistoryStore(dir)
	if store.db == nil {
		t.Fatal("expected a usable database")
	}
	store.RecordPendingMatch(MatchResult{
		MatchID:       42,
		MapID:         11,
		MapName:       "goldenpit.rms2",
		LeaderboardID: SoloRMRanked,
		QueueType:     "automatch2-6",
		Players:       []int32{100, 200},
		Teams:         []int32{0, 1},
	})
	_, _ = store.ApplyResult(MatchResult{
		MatchID: 42,
		Players: []int32{100, 200},
		Teams:   []int32{0, 1},
		Results: []int32{0, 1},
		Races:   []int32{27, 52},
	})
	_ = store.Close()

	reopened := NewMatchHistoryStore(dir)
	defer reopened.Close()
	if reopened.db == nil {
		t.Fatal("expected to reopen the database")
	}
	ratings := reopened.GetRatings([]int32{100, 200})
	if ratings[100].Wins != 0 || ratings[100].Losses != 1 {
		t.Fatalf("loser rating = %+v", ratings[100])
	}
	if ratings[200].Wins != 1 || ratings[200].Losses != 0 {
		t.Fatalf("winner rating = %+v", ratings[200])
	}
	if ratings[100].Rating != 984 || ratings[200].Rating != 1016 {
		t.Fatalf("unexpected elo: loser=%d winner=%d", ratings[100].Rating, ratings[200].Rating)
	}
}

func TestApplyResultIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	result := MatchResult{
		MatchID:       7,
		Players:       []int32{1, 2},
		Teams:         []int32{0, 1},
		Results:       []int32{1, 0},
		LeaderboardID: SoloRMRanked,
		QueueType:     "automatch2-6",
	}
	if _, rated := store.ApplyResult(result); !rated {
		t.Fatal("first report should rate the match")
	}
	first := store.GetRatings([]int32{1, 2})
	if _, rated := store.ApplyResult(result); rated {
		t.Fatal("second report must not rate the match again")
	}
	second := store.GetRatings([]int32{1, 2})
	for _, id := range []int32{1, 2} {
		if first[id].Rating != second[id].Rating || first[id].Games != second[id].Games {
			t.Fatalf("profile %d changed on duplicate report: %+v -> %+v", id, first[id], second[id])
		}
	}
	if !store.HasMatch(7) {
		t.Fatal("match should be reported")
	}
}

func TestApplyResultKeepsPendingMap(t *testing.T) {
	store := newTestStore(t)
	store.RecordPendingMatch(MatchResult{
		MatchID:       99,
		MapID:         11,
		MapName:       "goldenpit.rms2",
		LeaderboardID: SoloRMRanked,
		QueueType:     "automatch2-6",
		Players:       []int32{10, 20},
		Teams:         []int32{0, 1},
	})
	store.ApplyResult(MatchResult{
		MatchID: 99,
		Players: []int32{10, 20},
		Teams:   []int32{0, 1},
		Results: []int32{1, 0},
	})
	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].MapName != "goldenpit.rms2" || matches[0].MapID != 11 {
		t.Fatalf("map info lost: %+v", matches[0])
	}
	if len(matches[0].PlayerStats) != 2 {
		t.Fatalf("player stats = %+v", matches[0].PlayerStats)
	}
}

func TestCustomMatchDoesNotChangeRating(t *testing.T) {
	store := newTestStore(t)
	_, rated := store.ApplyResult(MatchResult{
		MatchID: 123,
		Players: []int32{1, 2},
		Teams:   []int32{0, 1},
		Results: []int32{1, 0},
	})
	if rated {
		t.Fatal("unranked match must not change ratings")
	}
	ratings := store.GetRatings([]int32{1, 2})
	for _, id := range []int32{1, 2} {
		if ratings[id].Rating != DefaultRating {
			t.Fatalf("profile %d rating = %d", id, ratings[id].Rating)
		}
	}
	if !store.HasMatch(123) {
		t.Fatal("custom match should still be stored")
	}
}

func TestLeaderboardPage(t *testing.T) {
	store := newTestStore(t)
	for i, matchID := range []int64{1, 2, 3} {
		store.ApplyResult(MatchResult{
			MatchID:       matchID,
			Players:       []int32{int32(100 + i), int32(200 + i)},
			Teams:         []int32{0, 1},
			Results:       []int32{1, 0},
			LeaderboardID: SoloRMRanked,
		})
	}
	page, total := store.GetLeaderboardPage(SoloRMRanked, 0, 10)
	if total != 6 || len(page) != 6 {
		t.Fatalf("total=%d len=%d", total, len(page))
	}
	if page[0].Rating < page[len(page)-1].Rating {
		t.Fatal("leaderboard is not sorted by rating desc")
	}
	if page[0].Rank != 1 {
		t.Fatalf("rank = %d", page[0].Rank)
	}
}
