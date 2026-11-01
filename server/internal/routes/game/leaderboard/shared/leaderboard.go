package shared

import (
	"encoding/json"
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// GetStatGroups builds the response shared by the Leaderboard endpoints:
//
//	[0, [statGroup...], [profileInfo...], [leaderboardStat...]]
//
// idsQuery is the JSON array received in profileids or statsids. When
// isProfileId is false the ids are stat group ids. includeLeaderboardStats
// controls whether the fourth array is filled with the rating rows of the
// requested ladder.
func GetStatGroups(r *http.Request, idsQuery string, isProfileId bool, includeLeaderboardStats bool) i.A {
	var ids []int32
	err := json.Unmarshal([]byte(idsQuery), &ids)
	if err != nil {
		return nil
	}
	game := models.G(r)
	users := game.Users()
	clientLibVersion := models.SessionOrPanic(r).GetClientLibVersion()
	leaderboardID := models.SoloRMRanked
	if raw := r.URL.Query().Get("leaderboard_id"); raw != "" {
		if parsed, parseErr := strconv.ParseInt(raw, 10, 32); parseErr == nil && parsed > 0 {
			leaderboardID = int32(parsed)
		}
	}

	seen := make(map[int32]struct{}, len(ids))
	profiles := make([]models.User, 0, len(ids))
	profileIDs := make([]int32, 0, len(ids))
	for _, id := range ids {
		var u models.User
		var ok bool
		if isProfileId {
			u, ok = users.GetUserById(id)
		} else {
			u, ok = users.GetUserByStatGroupID(id)
		}
		if !ok {
			continue
		}
		if _, duplicate := seen[u.GetId()]; duplicate {
			continue
		}
		seen[u.GetId()] = struct{}{}
		profiles = append(profiles, u)
		profileIDs = append(profileIDs, u.GetId())
	}

	ratings := game.MatchHistory().GetRatingsForLeaderboard(leaderboardID, profileIDs)

	statGroups := i.A{}
	profileInfos := i.A{}
	leaderboardStats := i.A{}
	for _, u := range profiles {
		rating, found := ratings[u.GetId()]
		if !found || rating == nil {
			rating = &models.PlayerRating{
				ProfileID:     u.GetId(),
				LeaderboardID: leaderboardID,
				Rating:        u.GetRating(),
				HighestRating: u.GetRating(),
			}
		} else if rating.Rating > 0 {
			u.SetRating(rating.Rating)
		}
		statGroups = append(statGroups, i.A{
			u.GetStatGroupID(),
			"",
			"",
			1,
			i.A{u.GetId()},
		})
		profileInfos = append(profileInfos, u.EncodeProfileInfo(clientLibVersion))
		if includeLeaderboardStats {
			leaderboardStats = append(leaderboardStats, u.EncodeLeaderboardStat(rating))
		}
	}

	return i.A{0, statGroups, profileInfos, leaderboardStats}
}
