package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// PersonalStatResponse is the body of GetPersonalStat. It has no rankTotal: the
// official endpoint answers one row per ladder instead of a page of one ladder.
type PersonalStatResponse struct {
	Result           shared.Result      `json:"result"`
	StatGroups       []shared.StatGroup `json:"statGroups"`
	LeaderboardStats []shared.Stat      `json:"leaderboardStats"`
}

// GetPersonalStat returns the ladder rows of the requested profiles, for every
// ladder the profile has a rating on.
//
// The official service answers one row per ladder for the same stat group (10
// rows for a sample profile, ladders 3,4,1,2,13,14,25,26,27,29 — C4) and no
// top-level rankTotal, so this handler deliberately does not use
// GetLeaderboardPage: a single default ladder would hide most of the data.
func GetPersonalStat(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	profiles := shared.ResolveProfiles(r, query.ProfileRequest())
	ratings := game.MatchHistory().GetAllRatingsForProfiles(shared.ProfileIDs(profiles))
	// No leaderboard_id is read: the official endpoint has no such parameter and
	// answers every ladder the profile is rated on. The title's default ladder is
	// only used for the single placeholder row of a profile with no rating
	// anywhere.
	defaultLeaderboard := models.DefaultLeaderboardFor(game.Title())

	response := PersonalStatResponse{
		Result:           shared.OK(),
		StatGroups:       make([]shared.StatGroup, 0, len(profiles)),
		LeaderboardStats: make([]shared.Stat, 0),
	}
	for _, profile := range profiles {
		rows := ratings[profile.ProfileID]
		if len(rows) == 0 {
			// No ladder row at all: emit a single row on the default ladder so
			// the caller still sees the profile instead of an empty array (§2.b).
			fallback := &models.PlayerRating{
				ProfileID:     profile.ProfileID,
				LeaderboardID: defaultLeaderboard,
				Rating:        models.DefaultRating,
				RankTotal:     -1,
			}
			statGroupID := shared.StatGroupIDOf(profile, fallback)
			response.StatGroups = append(response.StatGroups, shared.NewStatGroup(profile, statGroupID))
			response.LeaderboardStats = append(response.LeaderboardStats, shared.NewStat(statGroupID, fallback))
			continue
		}
		statGroupID := shared.StatGroupIDOf(profile, rows[0])
		response.StatGroups = append(response.StatGroups, shared.NewStatGroup(profile, statGroupID))
		for _, rating := range rows {
			response.LeaderboardStats = append(response.LeaderboardStats, shared.NewStat(statGroupID, rating))
		}
	}
	i.JSON(&w, response)
}
