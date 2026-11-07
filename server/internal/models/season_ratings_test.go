package models

import (
	"strings"
	"testing"
	"time"
)

// newSeasonStore builds a store whose clock the test controls, so a season
// boundary can be crossed without waiting three months.
func newSeasonStore(t *testing.T, gameID string, clock *time.Time) *MatchHistoryStore {
	t.Helper()
	dir := t.TempDir()
	store := NewMatchHistoryStore(dir)
	if store.db == nil {
		t.Fatalf("database was not created in %s", dir)
	}
	store.gameID = gameID
	store.now = func() time.Time { return *clock }
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func ratedResult(matchID int64, leaderboard int32, profiles []int32, teams []int32, results []int32) MatchResult {
	return MatchResult{
		MatchID:       matchID,
		LeaderboardID: leaderboard,
		// In the AoE4 overlay the ranked pool id matches its ladder id.
		QueueID:   leaderboard,
		MatchType: leaderboard,
		Rated:     true,
		Players:   profiles,
		Teams:     teams,
		Results:   results,
	}
}

func TestRankedSeasonResetsVisibleLadderButNotMMR(t *testing.T) {
	clock := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	store := newSeasonStore(t, "age4", &clock)
	season := CurrentRankedSeason(clock)

	ratings, rated := store.ApplyResult(ratedResult(100, 1, []int32{1, 2}, []int32{0, 1}, []int32{0, 1}))
	if !rated {
		t.Fatal("the match was not rated")
	}
	if ratings[2].Games != 1 || ratings[2].Wins != 1 {
		t.Fatalf("winner after one match = %+v", ratings[2])
	}
	mmr := store.GetMMR(1, 1, []int32{1, 2})
	if mmr[2] <= DefaultRating || mmr[1] >= DefaultRating {
		t.Fatalf("hidden MMR did not move: %v", mmr)
	}
	winnerMMR := mmr[2]

	// The next season resets the visible ladder...
	clock = season.End.Add(time.Hour)
	next := CurrentRankedSeason(clock)
	if next.Number == season.Number {
		t.Fatalf("clock did not cross a season boundary: %d", next.Number)
	}
	fresh := store.GetRatingsForLeaderboard(1, []int32{1, 2})
	if fresh[2].Rating != DefaultRating || fresh[2].Games != 0 || fresh[2].Wins != 0 {
		t.Fatalf("new season rating = %+v, want a fresh ladder", fresh[2])
	}
	page, total := store.GetLeaderboardPage(1, 0, 10)
	if total != 0 || len(page) != 0 {
		t.Fatalf("new season ladder has %d entries (total %d), want none", len(page), total)
	}

	// ...but the hidden MMR survives, so matchmaking keeps the skill.
	kept := store.GetMMR(1, 1, []int32{1, 2})
	if kept[2] != winnerMMR {
		t.Fatalf("MMR was reset at the season boundary: %d -> %d", winnerMMR, kept[2])
	}

	// Going back to the previous season still shows its own ladder.
	clock = season.Start.Add(time.Hour)
	previous := store.GetRatingsForLeaderboard(1, []int32{2})
	if previous[2].Games != 1 || previous[2].Wins != 1 {
		t.Fatalf("previous season ladder was lost: %+v", previous[2])
	}
}

func TestPlacementCalibrationEndsAtHiddenMMR(t *testing.T) {
	clock := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	store := newSeasonStore(t, "age4", &clock)

	for match := 0; match < PlacementMatches; match++ {
		result := ratedResult(int64(200+match), 1, []int32{1, 2}, []int32{0, 1}, []int32{0, 1})
		if _, rated := store.ApplyResult(result); !rated {
			t.Fatalf("placement match %d was not rated", match)
		}
	}
	ratings := store.GetRatingsForLeaderboard(1, []int32{1, 2})
	mmr := store.GetMMR(1, 1, []int32{1, 2})
	for _, profile := range []int32{1, 2} {
		if ratings[profile].Games != PlacementMatches {
			t.Fatalf("profile %d games = %d, want %d", profile, ratings[profile].Games, PlacementMatches)
		}
		if ratings[profile].Rating != mmr[profile] {
			t.Fatalf("profile %d visible rating = %d, want the hidden MMR %d after placements",
				profile, ratings[profile].Rating, mmr[profile])
		}
	}
	if mmr[2] <= mmr[1] {
		t.Fatalf("the winner's MMR %d is not above the loser's %d", mmr[2], mmr[1])
	}
}

func TestQuickMatchUpdatesHiddenMMRWithoutVisibleLadder(t *testing.T) {
	clock := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	store := newSeasonStore(t, "age4", &clock)

	// Queue 20 is the 1v1 Quick Match (unranked) pool: no visible ladder.
	result := MatchResult{
		MatchID: 300,
		QueueID: 20,
		Players: []int32{1, 2},
		Teams:   []int32{0, 1},
		Results: []int32{0, 1},
	}
	ratings, rated := store.ApplyResult(result)
	if rated {
		t.Fatal("an unranked queue must not move a visible ladder")
	}
	if ratings[2].Games != 0 {
		t.Fatalf("visible rating changed for an unranked queue: %+v", ratings[2])
	}
	mmr := store.GetMMR(20, 0, []int32{1, 2})
	if mmr[2] != DefaultRating+16 || mmr[1] != DefaultRating-16 {
		t.Fatalf("Quick Match MMR = %v, want +16/-16", mmr)
	}
	// The Quick Match MMR is its own: the ranked queue is untouched.
	rankedMMR := store.GetMMR(1, 1, []int32{1, 2})
	if rankedMMR[2] != DefaultRating {
		t.Fatalf("ranked MMR leaked from Quick Match: %v", rankedMMR)
	}

	// Re-reporting the same match must not move the MMR again.
	store.ApplyResult(result)
	again := store.GetMMR(20, 0, []int32{1, 2})
	if again[2] != mmr[2] {
		t.Fatalf("MMR moved on a duplicate report: %d -> %d", mmr[2], again[2])
	}
}

func TestSortCandidatesByMMR(t *testing.T) {
	base := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	oldestFar := &automatchPartyGroup{QueuedAt: base}
	closest := &automatchPartyGroup{QueuedAt: base.Add(10 * time.Second)}
	closerLater := &automatchPartyGroup{QueuedAt: base.Add(20 * time.Second)}
	candidates := []*automatchPartyGroup{oldestFar, closerLater, closest}
	mmr := map[*automatchPartyGroup]int32{
		oldestFar:   2000,
		closest:     1000,
		closerLater: 1050,
	}
	sortCandidatesByMMR(candidates, mmr, 1000)
	if candidates[0] != closest || candidates[1] != closerLater || candidates[2] != oldestFar {
		t.Fatalf("MMR ordering = %v", candidates)
	}

	// Equal distance falls back to FIFO.
	first := &automatchPartyGroup{QueuedAt: base}
	second := &automatchPartyGroup{QueuedAt: base.Add(time.Second)}
	tied := []*automatchPartyGroup{second, first}
	sortCandidatesByMMR(tied, map[*automatchPartyGroup]int32{first: 1000, second: 1100}, 1050)
	if tied[0] != first || tied[1] != second {
		t.Fatalf("FIFO tie-break = %v", tied)
	}
}

func TestSchemaHasSeasonalLadderAndHiddenMMR(t *testing.T) {
	store := newSeasonStore(t, "age4", new(time.Time))
	columns := map[string]string{}
	for _, table := range []string{"ratings", "mmr", "matches"} {
		rows, err := store.db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		var found []string
		for rows.Next() {
			var (
				cid, notNull, pk int
				name, columnType string
				defaultValue     any
			)
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				t.Fatal(err)
			}
			found = append(found, name)
		}
		_ = rows.Close()
		columns[table] = strings.Join(found, ",")
	}
	for table, column := range map[string]string{
		"ratings": "season",
		"mmr":     "queue_id",
		"matches": "mmr_applied",
	} {
		if !strings.Contains(columns[table], column) {
			t.Fatalf("%s is missing %q: %s", table, column, columns[table])
		}
	}
}

func TestTitlesWithoutSeasonsKeepTheirLadder(t *testing.T) {
	clock := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	store := newSeasonStore(t, "age2", &clock)

	ratings, rated := store.ApplyResult(ratedResult(400, SoloRMRanked, []int32{10, 20}, []int32{0, 1}, []int32{0, 1}))
	if !rated {
		t.Fatal("the match was not rated")
	}
	wonRating := ratings[20].Rating

	// Two years and several AoE4 seasons later the ladder is still there.
	clock = clock.Add(2 * 365 * 24 * time.Hour)
	kept := store.GetRatingsForLeaderboard(SoloRMRanked, []int32{20})
	if kept[20].Rating != wonRating || kept[20].Games != 1 || kept[20].Wins != 1 {
		t.Fatalf("age2 ladder was reset: %+v", kept[20])
	}
}
