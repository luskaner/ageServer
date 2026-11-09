package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// AvatarStatsResponse is the body of GetAvatarStatForProfile.
type AvatarStatsResponse struct {
	Result                shared.Result                `json:"result"`
	AvatarStatsForProfile []models.CommunityAvatarStat `json:"avatarStatsForProfile"`
}

// GetAvatarStatForProfile returns the avatar (career) statistics of the requested
// profiles as a flat array.
//
// The live service returns one flat list, not an array grouped per profile, and
// each entry omits the metadata blob the stored stat carries (C2).
//
// Limitation: avatar stats exist only for users the server has loaded, so an
// offline profile contributes no entries. The official service serves them from
// its database; this server has no per-profile stat table outside the account
// data of connected players (§2.c).
func GetAvatarStatForProfile(w http.ResponseWriter, r *http.Request) {
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	profiles := shared.ResolveProfiles(r, query.ProfileRequest())
	response := AvatarStatsResponse{
		Result:                shared.OK(),
		AvatarStatsForProfile: make([]models.CommunityAvatarStat, 0),
	}
	for _, profile := range profiles {
		if profile.User == nil {
			continue
		}
		avatarStats := profile.User.GetAvatarStats()
		if avatarStats == nil {
			continue
		}
		profileID := profile.ProfileID
		_ = avatarStats.WithReadOnly(func(data *models.AvatarStats) error {
			response.AvatarStatsForProfile = append(response.AvatarStatsForProfile, data.EncodeCommunity(profileID)...)
			return nil
		})
	}
	i.JSON(&w, response)
}
