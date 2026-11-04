package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// getStatGroupsByProfileIDsRequest mirrors the query string the client sends.
type getStatGroupsByProfileIDsRequest struct {
	ProfileIDs i.Json[[]int32] `schema:"profile_ids"`
}

// GetStatGroupsByProfileIDs returns stat groups for the given profile IDs.
// Public endpoint without a game session.
func GetStatGroupsByProfileIDs(w http.ResponseWriter, r *http.Request) {
	var req getStatGroupsByProfileIDsRequest
	if err := i.Bind(r, &req); err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	ids := normalizeProfileIDs(req.ProfileIDs.Data)

	game := models.G(r)
	users := game.Users()
	history := game.MatchHistory()

	statGroups := i.A{}
	for _, id := range ids {
		user, online := users.GetUserById(id)
		stored := history.GetProfiles([]int32{id})
		var profile *models.StoredProfile
		if len(stored) > 0 {
			profile = stored[id]
		}
		var statGroupID int32
		if online {
			statGroupID = user.GetStatGroupID()
		} else if profile != nil && profile.StatGroupID != 0 {
			statGroupID = profile.StatGroupID
		} else {
			statGroupID = id
		}
		statGroups = append(statGroups, i.A{
			statGroupID, // statGroupID
			"",          // statGroupName
			"",          // metaData
			1,           // type
			i.A{id},     // profileIDs
		})
	}
	i.JSON(&w, i.A{0, statGroups})
}