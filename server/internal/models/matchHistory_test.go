package models

import (
	"database/sql"
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

func TestMatchHistoryAddsReportColumnsToExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "automatch2.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE matches (
			match_id INTEGER PRIMARY KEY, map_id INTEGER NOT NULL DEFAULT 0, map_name TEXT NOT NULL DEFAULT '',
			leaderboard_id INTEGER NOT NULL DEFAULT 0, queue_type TEXT NOT NULL DEFAULT '',
			winner_team INTEGER NOT NULL DEFAULT -1, reported INTEGER NOT NULL DEFAULT 0,
			rated INTEGER NOT NULL DEFAULT 0, started_at INTEGER NOT NULL DEFAULT 0, finished_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE match_players (
			match_id INTEGER NOT NULL, profile_id INTEGER NOT NULL, team_id INTEGER NOT NULL DEFAULT 0,
			result INTEGER NOT NULL DEFAULT -1, race_id INTEGER NOT NULL DEFAULT 0, xp_gained INTEGER NOT NULL DEFAULT 0,
			rating_before INTEGER NOT NULL DEFAULT 0, rating_after INTEGER NOT NULL DEFAULT 0,
			rating_delta INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (match_id, profile_id)
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store := NewMatchHistoryStore(dir)
	if store.db == nil {
		t.Fatal("expected legacy database to open after migration")
	}
	defer store.Close()
	rows, err := store.db.Query(`SELECT player_order, counters, custom_updates FROM match_players`)
	if err != nil {
		t.Fatalf("report columns were not migrated: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
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

func TestApplyResultMatchesOfficialRatingDelta(t *testing.T) {
	store := newTestStore(t)
	for profileID, rating := range map[int32]int32{10: 716, 20: 717} {
		if _, err := store.db.Exec(
			`INSERT INTO ratings (leaderboard_id, profile_id, rating, highest_rating)
			 VALUES (?, ?, ?, ?)`,
			SoloRMRanked, profileID, rating, rating,
		); err != nil {
			t.Fatalf("seed rating for profile %d: %v", profileID, err)
		}
	}

	ratings, rated := store.ApplyResult(MatchResult{
		MatchID:       8,
		Players:       []int32{10, 20},
		Teams:         []int32{0, 1},
		Results:       []int32{1, 0},
		LeaderboardID: SoloRMRanked,
	})
	if !rated {
		t.Fatal("ranked result was not rated")
	}
	if ratings[10].Rating != 732 || ratings[20].Rating != 701 {
		t.Fatalf("ratings = %d and %d, want 732 and 701", ratings[10].Rating, ratings[20].Rating)
	}
	if ratings[10].RatingDelta != 16 || ratings[20].RatingDelta != -16 {
		t.Fatalf("rating deltas = %d and %d, want +16 and -16", ratings[10].RatingDelta, ratings[20].RatingDelta)
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

func TestApplyResultPersistsClientReportDetailsInReportOrder(t *testing.T) {
	store := newTestStore(t)
	store.RecordPendingMatch(MatchResult{
		MatchID:       100,
		MapName:       "Arabian Desert.rms",
		LeaderboardID: SoloRMRanked,
		QueueType:     "automatch2-6",
		Players:       []int32{10, 20},
		Teams:         []int32{0, 1},
	})
	store.ApplyResult(MatchResult{
		MatchID:       100,
		Players:       []int32{20, 10},
		Teams:         []int32{1, 0},
		Results:       []int32{0, 1},
		Counters:      []string{`{"units":2}`, `{"units":3}`},
		CustomUpdates: []string{`{"itemUpdates":["b"]}`, `{"itemUpdates":["a"]}`},
	})

	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 || len(matches[0].PlayerStats) != 2 {
		t.Fatalf("matches = %+v", matches)
	}
	first, second := matches[0].PlayerStats[0], matches[0].PlayerStats[1]
	if first.ProfileID != 20 || first.ReportOrder != 0 || first.Counters != `{"units":2}` || first.CustomUpdates != `{"itemUpdates":["b"]}` {
		t.Errorf("first reported player = %+v", first)
	}
	if second.ProfileID != 10 || second.ReportOrder != 1 || second.Counters != `{"units":3}` || second.CustomUpdates != `{"itemUpdates":["a"]}` {
		t.Errorf("second reported player = %+v", second)
	}
}

// The lobby metadata the client sends when hosting has to survive the match so
// getRecentMatchHistory can echo it back as encodedOptions/encodedSlots.
func TestApplyResultPersistsLobbyMetadata(t *testing.T) {
	store := newTestStore(t)
	store.ApplyResult(MatchResult{
		MatchID:        300,
		MapName:        "generated_map",
		MaxPlayers:     16,
		Description:    "unnamed_session",
		EncodedOptions: "eNq-options",
		EncodedSlots:   `12,[{"profileInfo.id":10}]`,
		Players:        []int32{10},
		Teams:          []int32{0},
		Results:        []int32{1},
	})

	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 {
		t.Fatalf("matches = %+v", matches)
	}
	got := matches[0]
	if got.MapName != "generated_map" || got.MaxPlayers != 16 || got.Description != "unnamed_session" {
		t.Errorf("metadata = %q/%d/%q", got.MapName, got.MaxPlayers, got.Description)
	}
	if got.EncodedOptions != "eNq-options" || got.EncodedSlots != `12,[{"profileInfo.id":10}]` {
		t.Errorf("blobs = %q/%q", got.EncodedOptions, got.EncodedSlots)
	}
}

// A second report of the same match without the lobby metadata must not wipe
// what the first one stored.
func TestApplyResultKeepsStoredLobbyMetadataOnRepeat(t *testing.T) {
	store := newTestStore(t)
	store.ApplyResult(MatchResult{
		MatchID:        301,
		MapName:        "generated_map",
		MaxPlayers:     16,
		Description:    "unnamed_session",
		EncodedOptions: "eNq-options",
		EncodedSlots:   "12,[]",
		Players:        []int32{10},
		Teams:          []int32{0},
		Results:        []int32{1},
	})
	store.ApplyResult(MatchResult{
		MatchID: 301,
		Players: []int32{10},
		Teams:   []int32{0},
		Results: []int32{1},
	})

	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 {
		t.Fatalf("matches = %+v", matches)
	}
	got := matches[0]
	if got.MapName != "generated_map" || got.MaxPlayers != 16 || got.Description != "unnamed_session" {
		t.Errorf("metadata = %q/%d/%q", got.MapName, got.MaxPlayers, got.Description)
	}
	if got.EncodedOptions != "eNq-options" || got.EncodedSlots != "12,[]" {
		t.Errorf("blobs = %q/%q", got.EncodedOptions, got.EncodedSlots)
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

func TestApplyResultKeepsStoredCountersOnRepeat(t *testing.T) {
	store := newTestStore(t)
	store.ApplyResult(MatchResult{
		MatchID:  200,
		Players:  []int32{10, 20},
		Teams:    []int32{0, 1},
		Results:  []int32{1, 0},
		Counters: []string{`{"villagers":12}`, `{"villagers":3}`},
	})
	// A re-report without counters sends empty values, which must preserve the
	// stored document instead of overwriting it with "{}".
	store.ApplyResult(MatchResult{
		MatchID:  200,
		Players:  []int32{10, 20},
		Teams:    []int32{0, 1},
		Results:  []int32{1, 0},
		Counters: []string{"", ""},
	})

	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	for _, player := range matches[0].PlayerStats {
		want := `{"villagers":12}`
		if player.ProfileID == 20 {
			want = `{"villagers":3}`
		}
		if player.Counters != want {
			t.Fatalf("profile %d counters = %q, want %q", player.ProfileID, player.Counters, want)
		}
	}
}

func TestApplyResultKeepsStoredCustomUpdatesOnRepeat(t *testing.T) {
	store := newTestStore(t)
	updates := []string{
		`{"itemUpdates":["item"],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`,
		`{"itemUpdates":[],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`,
	}
	store.ApplyResult(MatchResult{
		MatchID:       201,
		Players:       []int32{10, 20},
		Teams:         []int32{0, 1},
		Results:       []int32{1, 0},
		CustomUpdates: updates,
	})
	store.ApplyResult(MatchResult{
		MatchID:       201,
		Players:       []int32{10, 20},
		Teams:         []int32{0, 1},
		Results:       []int32{1, 0},
		CustomUpdates: []string{"", ""},
	})

	matches := store.GetRecentForProfiles([]int32{10}, 10)
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	for index, player := range matches[0].PlayerStats {
		if player.CustomUpdates != updates[index] {
			t.Fatalf("profile %d custom updates = %q, want %q", player.ProfileID, player.CustomUpdates, updates[index])
		}
	}
}

func TestResolveStatGroupIDPrecedence(t *testing.T) {
	cases := []struct {
		name         string
		user         int32
		stored       int32
		ratingStatID int32
		want         int32
	}{
		{name: "user wins", user: 11, stored: 22, ratingStatID: 33, want: 11},
		{name: "stored when offline", user: 0, stored: 22, ratingStatID: 33, want: 22},
		{name: "rating fallback", user: 0, stored: 0, ratingStatID: 33, want: 33},
		{name: "nothing known", user: 0, stored: 0, ratingStatID: 0, want: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ResolveStatGroupID(testCase.user, testCase.stored, testCase.ratingStatID); got != testCase.want {
				t.Fatalf("ResolveStatGroupID(%d, %d, %d) = %d, want %d",
					testCase.user, testCase.stored, testCase.ratingStatID, got, testCase.want)
			}
		})
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
