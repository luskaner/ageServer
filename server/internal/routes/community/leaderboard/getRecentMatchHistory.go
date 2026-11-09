package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// recentMatchHistoryLimit is how many games getRecentMatchHistory returns. The
// store caps the value at 100, so 30 is comfortably inside the limit (C12).
const recentMatchHistoryLimit = 30

// RecentMatchHistoryResponse is the body of getRecentMatchHistory.
type RecentMatchHistoryResponse struct {
	Result            shared.Result     `json:"result"`
	MatchHistoryStats []shared.Match    `json:"matchHistoryStats"`
	Profiles          []*shared.Profile `json:"profiles"`
}

// MatchHistoryResponse is the body of getMatchHistory. The top-level key is
// matchHistory, not matchHistoryStats: the live service uses a different name
// here than on getRecentMatchHistory (C5).
type MatchHistoryResponse struct {
	Result       shared.Result     `json:"result"`
	MatchHistory []shared.Match    `json:"matchHistory"`
	Profiles     []*shared.Profile `json:"profiles"`
}

// GetRecentMatchHistory returns the most recent reported games of the requested
// profiles, plus the union of every profile that appears in them.
//
// At least one identification parameter is required; without it the endpoint
// answers with empty arrays instead of scanning the whole history.
func GetRecentMatchHistory(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	profiles := shared.ResolveProfiles(r, query.ProfileRequest())
	if len(profiles) == 0 {
		i.JSON(&w, RecentMatchHistoryResponse{
			Result:            shared.OK(),
			MatchHistoryStats: []shared.Match{},
			Profiles:          []*shared.Profile{},
		})
		return
	}
	requested := shared.ProfileIDs(profiles)
	matches := game.MatchHistory().GetRecentForProfiles(requested, recentMatchHistoryLimit)

	// Every profile referenced by any match, so the caller can render the
	// players of all of them from a single response.
	profileIDs := append([]int32(nil), requested...)
	seen := make(map[int32]struct{}, len(requested))
	for _, id := range requested {
		seen[id] = struct{}{}
	}
	for _, match := range matches {
		for _, player := range match.PlayerStats {
			if _, duplicate := seen[player.ProfileID]; duplicate {
				continue
			}
			seen[player.ProfileID] = struct{}{}
			profileIDs = append(profileIDs, player.ProfileID)
		}
	}
	allProfiles := shared.ResolveProfiles(r, shared.ProfileRequest{ProfileIDs: profileIDs})

	history := make([]shared.Match, 0, len(matches))
	for _, match := range matches {
		history = append(history, encodeMatchObject(r, game, match, 0))
	}
	i.JSON(&w, RecentMatchHistoryResponse{
		Result:            shared.OK(),
		MatchHistoryStats: history,
		Profiles:          allProfiles,
	})
}

// encodeMatchObject builds one match object with its report-result, member and
// matchurl sub-arrays.
//
// gamemodID is 0 for getRecentMatchHistory (the official service does not report
// it there) and the real value on getMatchHistory.
func encodeMatchObject(r *http.Request, game models.Game, match models.MatchResult, gamemodID int32) shared.Match {
	store := game.MatchHistory()
	users := game.Users()

	profileIDs := make([]int32, 0, len(match.PlayerStats))
	for _, player := range match.PlayerStats {
		profileIDs = append(profileIDs, player.ProfileID)
	}
	ratings := store.GetRatingsForLeaderboard(match.LeaderboardID, profileIDs)
	stored := store.GetProfiles(profileIDs)

	reportResults := make([]shared.ReportResult, 0, len(match.PlayerStats))
	members := make([]shared.Member, 0, len(match.PlayerStats))
	for _, player := range match.PlayerStats {
		reportResults = append(reportResults, shared.NewReportResult(match, player))
		var userStatGroupID, storedStatGroupID, ratingStatGroupID int32
		if user, connected := users.GetUserById(player.ProfileID); connected {
			userStatGroupID = user.GetStatGroupID()
		}
		if entry, found := stored[player.ProfileID]; found && entry != nil {
			storedStatGroupID = entry.StatGroupID
		}
		rating := ratings[player.ProfileID]
		if rating != nil {
			ratingStatGroupID = rating.StatID
		}
		statGroupID := models.ResolveStatGroupID(userStatGroupID, storedStatGroupID, ratingStatGroupID)
		members = append(members, shared.NewMember(match, player, statGroupID, rating))
	}

	creator := int32(0)
	if len(match.PlayerStats) > 0 {
		creator = match.PlayerStats[0].ProfileID
	}
	urls := make([]shared.MatchURL, 0, 2)
	if cloudFiles := game.Resources().CloudFiles(); cloudFiles != nil && cloudFiles.ReplaysEnabled() {
		for _, kind := range []int{0, 1} {
			if key, entry, found := cloudFiles.FindReplay(match.MatchID, kind); found {
				urls = append(urls, shared.MatchURL{
					// FIXME(replay-uploader): the server does not persist which
					// profile uploaded a replay, so the creator is used as an
					// approximation. Phase 7 of the plan records the uploader.
					ProfileID: creator,
					URL:       replayURL(r, key),
					Size:      int64(entry.Length),
					DataType:  kind,
				})
			}
		}
	}
	return shared.NewMatch(match, reportResults, urls, members, gamemodID)
}

// replayURL builds the LAN URL of a stored replay. Unlike the official service
// the URL carries no SAS signature: the server does not sign cloud files, so
// there is no `sig` parameter (documented deviation, §2.f).
func replayURL(r *http.Request, key string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/cloudfiles/" + key
}
