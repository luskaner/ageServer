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
	if err := json.Unmarshal([]byte(idsQuery), &ids); err != nil {
		return nil
	}
	game := models.G(r)
	clientLibVersion := models.SessionOrPanic(r).GetClientLibVersion()
	leaderboardID := resolveLeaderboardID(r, game)
	profiles, profileIDs := collectProfiles(game.Users(), ids, isProfileId)
	statGroups, profileInfos, leaderboardStats := encodeStatRows(
		game, profiles, profileIDs, leaderboardID, clientLibVersion, includeLeaderboardStats,
	)
	return i.A{0, statGroups, profileInfos, leaderboardStats}
}

// GetPersonalStat builds the five-block response of getPersonalStat. Block 5
// is the avatar/personal stat row of each profile, which is why profiles that
// have never played still produce data.
func GetPersonalStat(r *http.Request, idsQuery string) i.A {
	var ids []int32
	if err := json.Unmarshal([]byte(idsQuery), &ids); err != nil {
		return nil
	}
	game := models.G(r)
	clientLibVersion := models.SessionOrPanic(r).GetClientLibVersion()
	leaderboardID := resolveLeaderboardID(r, game)
	profiles, profileIDs := collectProfiles(game.Users(), ids, true)
	statGroups, profileInfos, leaderboardStats := encodeStatRows(
		game, profiles, profileIDs, leaderboardID, clientLibVersion, true,
	)
	personalStats := i.A{}
	for _, u := range profiles {
		if avatarStats := u.GetAvatarStats(); avatarStats != nil {
			_ = avatarStats.WithReadOnly(func(data *models.AvatarStats) error {
				personalStats = append(personalStats, data.Encode(u.GetId())...)
				return nil
			})
		}
	}
	return i.A{0, statGroups, profileInfos, leaderboardStats, personalStats}
}

// resolveLeaderboardID returns the ladder the request asked for, or the game's
// default when the query names none.
func resolveLeaderboardID(r *http.Request, game models.Game) int32 {
	leaderboardID := models.DefaultLeaderboardFor(game.Title())
	if raw := r.URL.Query().Get("leaderboard_id"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 32); err == nil && parsed > 0 {
			leaderboardID = int32(parsed)
		}
	}
	return leaderboardID
}

// collectProfiles resolves the requested ids to users, dropping unknown ids and
// profiles that appear more than once, and returns the matching profile ids in
// the same order.
func collectProfiles(users models.Users, ids []int32, isProfileId bool) ([]models.User, []int32) {
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
	return profiles, profileIDs
}

// encodeStatRows builds the statGroup, profileInfo and leaderboardStat arrays
// for the given profiles. The rating rows are only filled when
// includeLeaderboardStats is true.
func encodeStatRows(game models.Game, profiles []models.User, profileIDs []int32, leaderboardID int32, clientLibVersion uint16, includeLeaderboardStats bool) (statGroups, profileInfos, leaderboardStats i.A) {
	ratings := game.MatchHistory().GetRatingsForLeaderboard(leaderboardID, profileIDs)
	statGroups = i.A{}
	profileInfos = i.A{}
	leaderboardStats = i.A{}
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
			leaderboardStats = append(leaderboardStats, models.EncodeLeaderboardStatForGame(game.Title(), u.GetStatGroupID(), u.GetRating(), rating))
		}
	}
	return
}
