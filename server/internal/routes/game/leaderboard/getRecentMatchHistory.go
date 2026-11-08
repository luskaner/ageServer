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
			reportResults = append(reportResults, matchHistoryReportResult(match, player))

			var userStatGroupID, storedStatGroupID int32
			var wins, losses, streak int32
			if user, ok := users.GetUserById(player.ProfileID); ok {
				userStatGroupID = user.GetStatGroupID()
			}
			if stored, ok := storedProfiles[player.ProfileID]; ok && stored != nil {
				storedStatGroupID = stored.StatGroupID
			}
			var ratingStatGroupID int32
			if rating, ok := ratings[player.ProfileID]; ok && rating != nil {
				ratingStatGroupID = rating.StatID
				wins = rating.Wins
				losses = rating.Losses
				streak = rating.Streak
			}
			statGroupID := models.ResolveStatGroupID(userStatGroupID, storedStatGroupID, ratingStatGroupID)
			// The official service never reports a zero rating or a zero
			// arbitration: an unrated match still carries the default rating and
			// arbitration 1, which is the value it uses for every single-player
			// match in the logs.
			oldRating, newRating := player.RatingBefore, player.RatingAfter
			if oldRating <= 0 {
				oldRating = models.DefaultRating
			}
			if newRating <= 0 {
				newRating = oldRating
			}
			memberStats = append(memberStats, i.A{
				match.MatchID,    // matchHistoryID
				player.ProfileID, // profileID
				player.RaceID,    // raceID
				statGroupID,      // statGroupID
				player.TeamID,    // teamID
				wins,             // wins
				losses,           // losses
				streak,           // streak
				1,                // arbitration
				player.Result,    // outcome
				oldRating,        // oldRating
				newRating,        // newRating
				1,                // reportType
			})
		}

		creator := int32(0)
		if len(match.PlayerStats) > 0 {
			creator = match.PlayerStats[0].ProfileID
		}
		mapName, maxPlayers, description, matchTypeID := matchHistoryMetadata(match)
		// matchUrls is one entry per uploaded artifact: age4 has kind 0 and 1,
		// age2 and athens (multiplayer) only kind 0. The URL carries no SAS; the
		// client asks getTempCredentials for it.
		matchURLs := i.A{}
		if store := game.Resources().CloudFiles(); store != nil && store.ReplaysEnabled() {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			for _, kind := range []int{0, 1} {
				if key, entry, ok := store.FindReplay(match.MatchID, kind); ok {
					url := scheme + "://" + r.Host + "/cloudfiles/" + key
					matchURLs = append(matchURLs, i.A{creator, url, entry.Length, kind})
				}
			}
		}
		history = append(history, i.A{
			match.MatchID,           // matchHistoryID
			creator,                 // creatorProfileID
			mapName,                 // mapName
			maxPlayers,              // maxPlayers
			matchTypeID,             // matchTypeID
			match.EncodedOptions,    // encodedOptions
			match.EncodedSlots,      // encodedSlots
			description,             // description
			match.StartedAt.Unix(),  // startGameTime
			match.FinishedAt.Unix(), // completionTime
			0,                       // observerTotal
			reportResults,           // matchHistoryReportResults
			i.A{},                   // matchHistoryItems
			matchURLs,               // matchUrls
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

// matchHistoryMetadata resolves the lobby metadata the client sent when hosting
// the advertisement. The official service echoes it back verbatim, so the client
// can rebuild the slot layout of the post-game screen.
func matchHistoryMetadata(match models.MatchResult) (mapName string, maxPlayers int32, description string, matchTypeID int32) {
	mapName = match.MapName
	description = match.Description
	// maxPlayers is the lobby capacity, not the number of players that showed
	// up: the official service reports 16 for every AoE IV match, 8 for AoE II
	// and 12 for AoM.
	maxPlayers = match.MaxPlayers
	if maxPlayers == 0 {
		maxPlayers = int32(len(match.PlayerStats))
	}
	if match.QueueType != "" || match.Rated {
		// Report the real automatch pool id instead of a fixed 26 (D13). Older
		// rows that predate the queue type keep 26 as a conservative value.
		matchTypeID = match.MatchType
		if queueID, ok := models.QueueIDFromQueueType(match.QueueType); ok {
			matchTypeID = queueID
		}
		if matchTypeID == 0 {
			matchTypeID = 26
		}
		description = "AUTOMATCH"
	}
	return mapName, maxPlayers, description, matchTypeID
}

func matchHistoryReportResult(match models.MatchResult, player models.MatchPlayer) i.A {
	return i.A{
		match.MatchID,    // matchHistoryID
		player.ProfileID, // profileID
		player.Result,    // resultType (1 = win)
		player.TeamID,    // teamID
		player.RaceID,    // raceID
		player.XPGained,  // xpGained
		matchPlayerCounters(player),
		matchPlayerCustomUpdates(player),
		match.StartedAt.Unix(), // matchStartDate
	}
}

func matchPlayerCounters(player models.MatchPlayer) string {
	if player.Counters == "" {
		return "{}"
	}
	return player.Counters
}

func matchPlayerCustomUpdates(player models.MatchPlayer) string {
	if player.CustomUpdates == "" {
		return `{"itemUpdates":[],"challengeUpdates":[],"eventChallengeUpdates":[],"clientUpdates":{}}`
	}
	return player.CustomUpdates
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
