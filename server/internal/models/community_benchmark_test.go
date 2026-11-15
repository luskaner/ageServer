package models

import (
	"testing"
	"time"
)

// BenchmarkGetLeaderBoard2_MaxPage measures a full 1000-row ladder page, the cap
// GetLeaderboardPageBy enforces (§7.6). Two orderings are compared because the
// default one is backed by idx_ratings_ladder while the wins ordering is not,
// which is exactly the case the plan asked to measure before deciding whether a
// second index is needed.
func BenchmarkGetLeaderBoard2_MaxPage(b *testing.B) {
	store := NewMatchHistoryStore(b.TempDir())
	b.Cleanup(func() { _ = store.Close() })

	// 1200 profiles, each with a stored rating on the same ladder, so the page
	// is always full and the population is larger than one page.
	const profiles = 1200
	if store.db == nil {
		b.Fatal("database was not created")
	}
	now := time.Now().Unix()
	for id := int32(1); id <= profiles; id++ {
		if _, err := store.db.Exec(
			`INSERT INTO ratings (leaderboard_id, season, profile_id, stat_id, rating, highest_rating,
			                     games, wins, losses, streak, last_match)
			 VALUES (?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			SoloRMRanked, id, id, 1000+int(id%500), 1000+int(id%500),
			int(id%50), int(id%40), int(id%10), int(id%5), now,
		); err != nil {
			b.Fatalf("seed profile %d: %v", id, err)
		}
	}

	b.Run("sortBy=rating", func(b *testing.B) {
		for b.Loop() {
			page, total := store.GetLeaderboardPageBy(SoloRMRanked, 0, MaxLeaderboardPageSize, 1)
			if len(page) != MaxLeaderboardPageSize || total != profiles {
				b.Fatalf("page=%d total=%d", len(page), total)
			}
		}
	})
	b.Run("sortBy=wins", func(b *testing.B) {
		for b.Loop() {
			page, total := store.GetLeaderboardPageBy(SoloRMRanked, 0, MaxLeaderboardPageSize, 0)
			if len(page) != MaxLeaderboardPageSize || total != profiles {
				b.Fatalf("page=%d total=%d", len(page), total)
			}
		}
	})
}
