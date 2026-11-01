package item

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type getInventoryByProfileIDsRequest struct {
	ProfileIDs i.Json[[]int32] `json:"profileIDs"`
}

func GetInventoryByProfileIDs(w http.ResponseWriter, r *http.Request) {
	var req getInventoryByProfileIDsRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	initialData := make(i.A, len(req.ProfileIDs.Data))
	finalData := make(i.A, len(req.ProfileIDs.Data))
	game := models.G(r)
	locations := game.Items().EncodeLocations()
	users := game.Users()
	sess := models.SessionOrPanic(r)
	userId := sess.GetUserId()
	for j, profileId := range req.ProfileIDs.Data {
		var itemsEncoded i.A
		// Only return items for the user's own profile to avoid crash (AoE4) when looking another's player profile
		// Make it for all games as a precaution
		if userId == profileId {
			if u, ok := users.GetUserById(profileId); ok {
				_ = u.GetItems().WithReadOnly(func(data *map[int32]*models.MainItem) error {
					for _, item := range *data {
						// FIXME: Not all items should be shared with all users
						itemsEncoded = append(itemsEncoded, item.Encode(profileId))
					}
					return nil
				})
			}
		}
		profileIdStr := strconv.Itoa(int(profileId))
		initialData[j] = i.A{
			profileIdStr, // profileID
			itemsEncoded, // items
		}
		finalData[j] = i.A{
			profileIdStr, // profileID
			locations,    // item locations
		}
	}
	i.JSON(&w, i.A{0, initialData, finalData})
}
