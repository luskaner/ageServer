package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// getLeaderboard2Query mirrors the query string of the ladder listing. Every
// value is a scalar, so plain schema tags are enough.
type getLeaderboard2Query struct {
	LeaderboardID int32  `schema:"leaderboard_id"`
	Platform      string `schema:"platform"`
	SortBy        int    `schema:"sortBy"`
	Start         int    `schema:"start"`
	Count         int    `schema:"count"`
}

// LeaderBoard2Response is the body of getLeaderBoard2.
type LeaderBoard2Response struct {
	Result           shared.Result      `json:"result"`
	StatGroups       []shared.StatGroup `json:"statGroups"`
	LeaderboardStats []shared.Stat      `json:"leaderboardStats"`
	RankTotal        int64              `json:"rankTotal"`
}

// GetLeaderBoard2 returns one page of a ladder as objects.
//
// Normalisation follows the game route it mirrors (getLeaderBoard.go): the
// ladder defaults to the title's default, start is 1-based in the API and 0-based
// in the store, and count is capped by models.MaxLeaderboardPageSize. The page
// is always the ladder of the active season, because the store filters by season
// (C12).
func GetLeaderBoard2(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	defaultLeaderboard := models.DefaultLeaderboardFor(game.Title())
	q := getLeaderboard2Query{LeaderboardID: defaultLeaderboard, Start: 1, Count: 100}
	shared.BindQuery(r, &q)
	if q.LeaderboardID <= 0 {
		q.LeaderboardID = defaultLeaderboard
	}
	if q.Start <= 0 {
		q.Start = 1
	}
	if q.Count <= 0 {
		q.Count = 100
	}
	if q.Count > models.MaxLeaderboardPageSize {
		q.Count = models.MaxLeaderboardPageSize
	}

	page, total := game.MatchHistory().GetLeaderboardPageBy(q.LeaderboardID, q.Start-1, q.Count, q.SortBy)
	response := LeaderBoard2Response{
		Result:           shared.OK(),
		StatGroups:       make([]shared.StatGroup, 0, len(page)),
		LeaderboardStats: make([]shared.Stat, 0, len(page)),
		RankTotal:        total,
	}
	// Resolve every id through the same precedence as the game ladder: the
	// connected user, then the stored identity, then a placeholder.
	pageIDs := make([]int32, 0, len(page))
	for _, rating := range page {
		if rating != nil {
			pageIDs = append(pageIDs, rating.ProfileID)
		}
	}
	profiles := shared.ResolveProfiles(r, shared.ProfileRequest{ProfileIDs: pageIDs})
	byID := make(map[int32]*shared.Profile, len(profiles))
	for _, profile := range profiles {
		byID[profile.ProfileID] = profile
	}

	for _, rating := range page {
		if rating == nil {
			continue
		}
		profile := byID[rating.ProfileID]
		if profile == nil {
			// Resolution always yields at least a placeholder; this is defensive
			// so a rating row is never dropped silently.
			profile = &shared.Profile{ProfileID: rating.ProfileID}
		}
		statGroupID := shared.StatGroupIDOf(profile, rating)
		// The row is emitted next to its stat group, so this server answers
		// aligned arrays where the official service does not (C17). Clients must
		// join by statgroup_id, never by index.
		response.StatGroups = append(response.StatGroups, shared.NewStatGroup(profile, statGroupID))
		response.LeaderboardStats = append(response.LeaderboardStats, shared.NewStat(statGroupID, rating))
	}
	i.JSON(&w, response)
}
