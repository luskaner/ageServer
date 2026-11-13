package shared

import "github.com/luskaner/ageServer/server/internal/models"

// Stat is the leaderboardStats[] row of the public API.
//
// The wiki documents 14 fields; the live service returns 18 (verified
// 2026-11-08). models.EncodeLeaderboardStatForGame is not usable here: it is the
// positional 16/19-field layout the game client parses, with a different order
// (§3.3, C13).
type Stat struct {
	StatGroupID   int32 `json:"statgroup_id"`
	LeaderboardID int32 `json:"leaderboard_id"`
	Wins          int32 `json:"wins"`
	Losses        int32 `json:"losses"`
	// Streak is positive for a winning streak and negative for a losing one.
	Streak int32 `json:"streak"`
	// Disputes and Drops are not recorded by this server; the official service
	// does, and the field must still be present (§3.3).
	Disputes int32 `json:"disputes"`
	Drops    int32 `json:"drops"`
	// Rank is -1 when the profile holds no rank on this ladder.
	Rank int64 `json:"rank"`
	// RankTotal keeps the store's sentinels untouched: -1 means "unknown
	// ladder", 0 "empty ladder" and a positive value is a real population
	// (§3.3). Do not normalise it to 0.
	RankTotal int64 `json:"ranktotal"`
	RankLevel int32 `json:"ranklevel"`
	Rating    int32 `json:"rating"`
	// RegionRank and RegionRankTotal are filled by the official service; this
	// server has no per-region ladder, so -1 is sent (documented deviation).
	RegionRank      int64 `json:"regionrank"`
	RegionRankTotal int64 `json:"regionranktotal"`
	LastMatchDate   int64 `json:"lastmatchdate"`
	// Historical peaks are not persisted; 0 is the documented deviation.
	HighestRank      int64  `json:"highestrank"`
	HighestRankLevel int32  `json:"highestranklevel"`
	HighestRating    int32  `json:"highestrating"`
	Metadata         string `json:"metadata"`
}

// NewStat builds the row of a profile's standing on one ladder.
func NewStat(statGroupID int32, r *models.PlayerRating) Stat {
	stat := Stat{
		StatGroupID:      statGroupID,
		Disputes:         0,
		Drops:            0,
		Rank:             -1,
		RankTotal:        -1,
		RegionRank:       -1,
		RegionRankTotal:  -1,
		HighestRank:      0,
		HighestRankLevel: 0,
		Metadata:         "",
	}
	if r == nil {
		stat.Rating = models.DefaultRating
		stat.RankLevel = RankLevel(nil)
		return stat
	}
	stat.LeaderboardID = r.LeaderboardID
	stat.Wins = r.Wins
	stat.Losses = r.Losses
	stat.Streak = r.Streak
	stat.RankTotal = r.RankTotal
	if r.Rank > 0 {
		stat.Rank = r.Rank
	}
	stat.Rating = r.Rating
	if stat.Rating <= 0 {
		stat.Rating = models.DefaultRating
	}
	stat.HighestRating = r.HighestRating
	if !r.LastMatch.IsZero() {
		stat.LastMatchDate = r.LastMatch.Unix()
	}
	stat.RankLevel = RankLevel(r)
	return stat
}

// RankLevel derives the ranklevel field, which has three possible values in the
// live API (§3.3, matiz C13):
//
//	-1  the ladder population is unknown (ranktotal < 0),
//	 1  the profile holds a positive rank,
//	 0  the profile has a rating row but no rank yet.
func RankLevel(r *models.PlayerRating) int32 {
	if r == nil || r.RankTotal < 0 {
		return -1
	}
	if r.Rank > 0 {
		return 1
	}
	return 0
}
