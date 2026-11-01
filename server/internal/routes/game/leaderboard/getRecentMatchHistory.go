package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

// getRecentMatchHistoryRequest mirrors the query string the client sends.
// profile_ids is a JSON array, which i.Json decodes transparently.
type getRecentMatchHistoryRequest struct {
	ProfileIDs i.Json[[]int32] `schema:"profile_ids"`
}

// GetRecentMatchHistory returns the latest reported games of the requested
// profiles.
//
// The client deserialises MatchHistoryStats positionally, so the layout below
// must match the current client exactly:
//
//	[matchHistoryID, creatorProfileID, mapName, maxPlayers, matchTypeID,
//	 encodedOptions, encodedSlots, description, startGameTime, completionTime,
//	 observerTotal, matchHistoryReportResults, matchHistoryItems, matchUrls,
//	 matchVersionInfo, matchHistoryMemberStats]
func GetRecentMatchHistory(w http.ResponseWriter, r *http.Request) {
	logger.Printf("leaderboard.getRecentMatchHistory: query=%s\n", r.URL.RawQuery)

	var req getRecentMatchHistoryRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("leaderboard.getRecentMatchHistory: could not bind query: %s\n", err)
	}
	requested := normalizeProfileIDs(req.ProfileIDs.Data)

	game := models.G(r)
	store := game.MatchHistory()
	users := game.Users()
	matches := store.GetRecentForProfiles(requested, 30)

	history := make(i.A, 0, len(matches))
	for _, match := range matches {
		profiles := make([]int32, 0, len(match.PlayerStats))
		for _, player := range match.PlayerStats {
			profiles = append(profiles, player.ProfileID)
		}
		ratings := store.GetRatingsForLeaderboard(match.LeaderboardID, profiles)
		storedProfiles := store.GetProfiles(profiles)

		reportResults := make(i.A, 0, len(match.PlayerStats))
		memberStats := make(i.A, 0, len(match.PlayerStats))
		for _, player := range match.PlayerStats {
			reportResults = append(reportResults, i.A{
				match.MatchID,    // matchHistoryID
				player.ProfileID, // profileID
				player.Result,    // resultType (1 = win)
				player.TeamID,    // teamID
				player.RaceID,    // raceID
				player.XPGained,  // xpGained
				"{}",             // counters
				`{"itemUpdates":[],"challengeUpdates":[],"clientUpdates":{}}`, // customUpdates
				match.StartedAt.Unix(), // matchStartDate
			})

			statGroupID := int32(0)
			var wins, losses, streak int32
			if user, ok := users.GetUserById(player.ProfileID); ok {
				statGroupID = user.GetStatGroupID()
			} else if stored, ok := storedProfiles[player.ProfileID]; ok {
				statGroupID = stored.StatGroupID
			}
			if rating, ok := ratings[player.ProfileID]; ok && rating != nil {
				if rating.StatID != 0 && statGroupID == 0 {
					statGroupID = rating.StatID
				}
				wins = rating.Wins
				losses = rating.Losses
				streak = rating.Streak
			}
			memberStats = append(memberStats, i.A{
				match.MatchID,       // matchHistoryID
				player.ProfileID,    // profileID
				player.RaceID,       // raceID
				statGroupID,         // statGroupID
				player.TeamID,       // teamID
				wins,                // wins
				losses,              // losses
				streak,              // streak
				0,                   // arbitration
				player.Result,       // outcome
				player.RatingBefore, // oldRating
				player.RatingAfter,  // newRating
				0,                   // reportType
			})
		}

		creator := int32(0)
		if len(match.PlayerStats) > 0 {
			creator = match.PlayerStats[0].ProfileID
		}
		matchTypeID := 0
		description := match.MapName
		if match.QueueType != "" || match.Rated {
			// 26 is the 1v1 ranked automatch type used by the official service.
			matchTypeID = 26
			description = "AUTOMATCH"
		}
		history = append(history, i.A{
			match.MatchID,           // matchHistoryID
			creator,                 // creatorProfileID
			match.MapName,           // mapName
			len(match.PlayerStats),  // maxPlayers
			matchTypeID,             // matchTypeID
			"",                      // encodedOptions
			"",                      // encodedSlots
			description,             // description
			match.StartedAt.Unix(),  // startGameTime
			match.FinishedAt.Unix(), // completionTime
			0,                       // observerTotal
			reportResults,           // matchHistoryReportResults
			i.A{},                   // matchHistoryItems
			i.A{},                   // matchUrls
			i.A{"INVALID", "INVALID", "INVALID", 0, 0, 0, 0}, // matchVersionInfo
			memberStats, // matchHistoryMemberStats
		})
	}

	logger.Printf(
		"leaderboard.getRecentMatchHistory: requested=%v matches=%d\n",
		requested,
		len(history),
	)
	i.JSON(&w, i.A{0, history})
}

// normalizeProfileIDs drops the placeholder identifiers the client appends to
// pad its request and de-duplicates the result while preserving the order.
func normalizeProfileIDs(ids []int32) []int32 {
	seen := make(map[int32]struct{}, len(ids))
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
