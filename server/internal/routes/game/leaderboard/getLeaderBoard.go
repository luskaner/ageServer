package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

// getLeaderBoardRequest mirrors the query string the client sends. Binding is
// done through the shared decoder, so unknown parameters are ignored.
type getLeaderBoardRequest struct {
	LeaderboardID int32 `schema:"leaderboard_id"`
	Start         int   `schema:"start"`
	Count         int   `schema:"count"`
}

// GetLeaderBoard returns one page of a ladder.
//
// The client sends leaderboard_id, start (1-based), count and sortBy. The
// response follows the positional layout of LeaderboardGetLeaderboardEvent:
//
//	[result, [statGroup...], [profileInfo...], [leaderboardStat...]]
//
// Each statGroup is [statGroupID, statGroupName, metaData, type, [profileID...]]
// and each leaderboardStat is produced by MainUser.EncodeLeaderboardStat.
//
// The three arrays are matched by index, so an entry is only emitted when all
// three positions can be filled.
func GetLeaderBoard(w http.ResponseWriter, r *http.Request) {
	logger.Printf("leaderboard.getLeaderBoard: query=%s\n", r.URL.RawQuery)

	// Defaults are assigned before binding so a missing parameter keeps the
	// value the client expects: the 1v1 RM ranked ladder, first page, 100
	// entries per page.
	q := getLeaderBoardRequest{
		LeaderboardID: models.SoloRMRanked,
		Start:         1,
		Count:         100,
	}
	if err := i.Bind(r, &q); err != nil {
		logger.Printf("leaderboard.getLeaderBoard: could not bind query: %s\n", err)
		i.JSON(&w, i.A{2, i.A{}, i.A{}, i.A{}})
		return
	}
	if q.LeaderboardID <= 0 {
		q.LeaderboardID = models.SoloRMRanked
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

	game := models.G(r)
	page, total := game.MatchHistory().GetLeaderboardPage(q.LeaderboardID, q.Start-1, q.Count)
	users := game.Users()
	history := game.MatchHistory()
	clientLibVersion := models.SessionOrPanic(r).GetClientLibVersion()

	// Ratings only reference profile IDs. The stored identity lets the ladder
	// list players that are not connected right now, which is what the client
	// expects: entries must not disappear just because a player logged out.
	pageIDs := make([]int32, 0, len(page))
	for _, rating := range page {
		pageIDs = append(pageIDs, rating.ProfileID)
	}
	stored := history.GetProfiles(pageIDs)

	statGroups := i.A{}
	profiles := i.A{}
	stats := i.A{}
	live := 0
	offline := 0
	skipped := 0
	for _, rating := range page {
		user, connected := users.GetUserById(rating.ProfileID)
		if connected {
			user.SetRating(rating.Rating)
			statGroups = append(statGroups, i.A{
				user.GetStatGroupID(), // statGroupID
				"",                    // statGroupName
				"",                    // metaData
				1,                     // type
				i.A{user.GetId()},
			})
			profiles = append(profiles, user.EncodeProfileInfo(clientLibVersion))
			stats = append(stats, user.EncodeLeaderboardStat(rating))
			live++
			continue
		}

		profile := stored[rating.ProfileID]
		if profile == nil || profile.Alias == "" {
			// The player has a rating but the server has never stored its
			// identity (ratings recorded before profiles were persisted). Emit
			// a placeholder row so the entry is not lost; the real alias
			// replaces it as soon as the player connects again.
			profile = models.PlaceholderProfile(rating.ProfileID, rating.StatID)
			skipped++
		} else {
			offline++
		}

		statGroupID := profile.StatGroupID
		if statGroupID == 0 {
			statGroupID = rating.StatID
		}
		if statGroupID == 0 {
			statGroupID = rating.ProfileID
		}
		statGroups = append(statGroups, i.A{
			statGroupID, // statGroupID
			"",          // statGroupName
			"",          // metaData
			1,           // type
			i.A{profile.ProfileID},
		})
		profiles = append(profiles, models.EncodeStoredProfileInfo(profile, clientLibVersion))
		stats = append(stats, models.EncodeLeaderboardStatFor(statGroupID, rating.Rating, rating))
	}
	logger.Printf(
		"leaderboard.getLeaderBoard: leaderboard=%d start=%d count=%d total=%d returned=%d live=%d offline=%d placeholder=%d\n",
		q.LeaderboardID,
		q.Start,
		q.Count,
		total,
		len(page),
		live,
		offline,
		skipped,
	)
	i.JSON(&w, i.A{0, statGroups, profiles, stats})
}
