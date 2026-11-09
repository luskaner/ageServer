package models

import (
	"testing"
	"time"
)

// recordRatedMatch stores a reported 1v1 result so the ladder has a row for both
// profiles.
func recordRatedMatch(t *testing.T, store *MatchHistoryStore, matchID int64, winner, loser int32) {
	t.Helper()
	store.RecordMatch(MatchResult{
		MatchID:       matchID,
		LeaderboardID: SoloRMRanked,
		FinishedAt:    time.Unix(matchID, 0),
		Players:       []int32{winner, loser},
		Teams:         []int32{1, 2},
		Results:       []int32{1, 0},
		Races:         []int32{1, 2},
	})
}

// TestGetLeaderboardPageBy_SortsByWinsAndRating checks the two orderings the
// public getLeaderBoard2 exposes, and that both report the same rankTotal.
func TestGetLeaderboardPageBy_SortsByWinsAndRating(t *testing.T) {
	store := newTestStore(t)
	// Profile 1 wins three matches, profile 2 wins one. Rating and wins
	// therefore rank the two profiles the same way; a second pair is needed to
	// prove the orderings are actually different, so profile 3 is given many
	// wins against a weak opponent.
	for id := int64(1); id <= 4; id++ {
		recordRatedMatch(t, store, id, 1, 2)
	}
	for id := int64(10); id <= 16; id++ {
		recordRatedMatch(t, store, id, 3, 4)
	}

	byRating, ratingTotal := store.GetLeaderboardPageBy(SoloRMRanked, 0, 100, 1)
	byWins, winsTotal := store.GetLeaderboardPageBy(SoloRMRanked, 0, 100, 0)
	if ratingTotal != winsTotal {
		t.Fatalf("rankTotal differs between orderings: rating=%d wins=%d", ratingTotal, winsTotal)
	}
	if ratingTotal != 4 {
		t.Fatalf("rankTotal = %d, want 4", ratingTotal)
	}
	if len(byRating) != len(byWins) {
		t.Fatalf("page sizes differ: rating=%d wins=%d", len(byRating), len(byWins))
	}
	// The wins ordering must be monotonic in wins.
	for index := 1; index < len(byWins); index++ {
		if byWins[index-1].Wins < byWins[index].Wins {
			t.Fatalf("wins ordering is not descending: %d then %d",
				byWins[index-1].Wins, byWins[index].Wins)
		}
	}
	// And the rating ordering monotonic in rating.
	for index := 1; index < len(byRating); index++ {
		if byRating[index-1].Rating < byRating[index].Rating {
			t.Fatalf("rating ordering is not descending: %d then %d",
				byRating[index-1].Rating, byRating[index].Rating)
		}
	}
	// Ranks are 1-based and follow the page position.
	if byWins[0].Rank != 1 {
		t.Fatalf("first rank = %d, want 1", byWins[0].Rank)
	}
}

// TestGetLeaderboardPage_CapsCount checks the MaxLeaderboardPageSize bound is
// still enforced through the new sort-aware entry point.
func TestGetLeaderboardPage_CapsCount(t *testing.T) {
	store := newTestStore(t)
	recordRatedMatch(t, store, 1, 1, 2)
	page, _ := store.GetLeaderboardPageBy(SoloRMRanked, 0, MaxLeaderboardPageSize*10, 1)
	if len(page) > MaxLeaderboardPageSize {
		t.Fatalf("page has %d rows, above the cap %d", len(page), MaxLeaderboardPageSize)
	}
}

// TestGetAllRatingsForProfiles checks that every ladder of a profile is returned,
// not just the default one, which is what GetPersonalStat needs (C4).
func TestGetAllRatingsForProfiles(t *testing.T) {
	store := newTestStore(t)
	// Profile 1 plays on the 1v1 RM ladder and then on a second one.
	store.RecordMatch(MatchResult{
		MatchID:       1,
		LeaderboardID: SoloRMRanked,
		FinishedAt:    time.Unix(1, 0),
		Players:       []int32{1, 2},
		Teams:         []int32{1, 2},
		Results:       []int32{1, 0},
		Races:         []int32{1, 2},
	})
	store.RecordMatch(MatchResult{
		MatchID:       2,
		LeaderboardID: 4,
		FinishedAt:    time.Unix(2, 0),
		Players:       []int32{1, 3},
		Teams:         []int32{1, 2},
		Results:       []int32{1, 0},
		Races:         []int32{1, 2},
	})

	all := store.GetAllRatingsForProfiles([]int32{1})
	rows := all[1]
	if len(rows) != 2 {
		t.Fatalf("profile 1 has %d ladder rows, want 2 (ladders %d and 4)", len(rows), SoloRMRanked)
	}
	seen := make(map[int32]bool, len(rows))
	for _, row := range rows {
		seen[row.LeaderboardID] = true
		// ranktotal is the population of that ladder, so it must be positive on
		// a ladder with a stored result.
		if row.RankTotal <= 0 {
			t.Errorf("ladder %d ranktotal = %d, want the population", row.LeaderboardID, row.RankTotal)
		}
		// The rank is the position by rating, not a sentinel: a profile that
		// just won must be ranked, so ranklevel can resolve to 1 instead of 0.
		if row.Rank <= 0 || row.Rank > row.RankTotal {
			t.Errorf("ladder %d rank = %d, outside 1..%d", row.LeaderboardID, row.Rank, row.RankTotal)
		}
	}
	if !seen[SoloRMRanked] || !seen[4] {
		t.Fatalf("ladders = %v, want both %d and 4", seen, SoloRMRanked)
	}
	// A profile with no row at all yields no entry.
	if rows := store.GetAllRatingsForProfiles([]int32{999})[999]; len(rows) != 0 {
		t.Fatalf("unknown profile has %d rows, want 0", len(rows))
	}
}

// TestGetProfileByPlatformUserID checks the inverse lookup the public API needs
// to turn /steam/<id> into a profile id.
func TestGetProfileByPlatformUserID(t *testing.T) {
	store := newTestStore(t)
	store.UpsertProfile(StoredProfile{
		ProfileID:      42,
		StatGroupID:    200,
		Alias:          "Oni.TheViper",
		PlatformUserID: 76561197984749679,
		XP:             9667,
		Level:          3,
		Region:         2,
	})

	profile, found := store.GetProfileByPlatformUserID(76561197984749679, false)
	if !found {
		t.Fatal("steam profile was not found")
	}
	if profile.ProfileID != 42 || profile.Alias != "Oni.TheViper" || profile.Level != 3 {
		t.Fatalf("profile = %+v", profile)
	}
	// The Xbox bit is part of the key: the same number must not resolve.
	if _, found := store.GetProfileByPlatformUserID(76561197984749679, true); found {
		t.Fatal("a steam id resolved as an xbox id")
	}
	if _, found := store.GetProfileByPlatformUserID(0, false); found {
		t.Fatal("id 0 resolved to a profile")
	}
}

// TestGetUserByAlias covers the alias lookup Fase 0 adds for the aliases
// parameter. Only a connected user has an alias, so the lookup has to go through
// the user store rather than the profiles table.
func TestGetUserByAlias(t *testing.T) {
	users := &MainUsers{}
	users.Initialize()
	// GenerateFn is replaced so no per-user file store is needed.
	users.GenerateFn = func(_ string, _ *PersistentStringJsonMap, _ Items, _ AvatarStatDefinitions,
		_ string, _ bool, _ uint64, alias string) User {
		return &MainUser{alias: alias, id: 7}
	}
	user := users.GetOrCreateUser("age2", nil, nil, "127.0.0.1:1", "00:11:22:33:44:55", false, 5, "Needle")
	if user == nil {
		t.Fatal("could not create the user")
	}
	found, ok := users.GetUserByAlias("Needle")
	if !ok || found.GetId() != user.GetId() {
		t.Fatalf("GetUserByAlias = %+v, %v", found, ok)
	}
	if _, ok := users.GetUserByAlias("missing"); ok {
		t.Fatal("unknown alias resolved")
	}
	if _, ok := users.GetUserByAlias(""); ok {
		t.Fatal("empty alias resolved")
	}
}
