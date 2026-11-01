package item

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type detachItemsRequest struct {
	itemRequest
	LocationIds      i.Json[[]int32] `schema:"itemLocations"`
	DurabilityCounts i.Json[[]int32] `schema:"itemCharges"`
}

func DetachItems(w http.ResponseWriter, r *http.Request) {
	var req detachItemsRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	minLength := min(len(req.ItemIds.Data), len(req.LocationIds.Data), len(req.DurabilityCounts.Data))
	maxLength := max(len(req.ItemIds.Data), len(req.LocationIds.Data), len(req.DurabilityCounts.Data))
	if minLength == 0 || maxLength == 0 || minLength != maxLength {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	u, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	errorCodes := make([]int, minLength)
	itemsEncoded := make([]i.A, minLength)
	_ = u.GetItems().WithReadWrite(func(data *map[int32]*models.MainItem) error {
		for j, itemId := range req.ItemIds.Data {
			locationID := req.LocationIds.Data[j]
			durability := req.DurabilityCounts.Data[j]
			if item, exists := (*data)[itemId]; !exists {
				errorCodes[j] = 2
				itemsEncoded[j] = i.A{}
			} else {
				errorCodes[j] = 0
				if locationID != -1 {
					item.SetLocationID(locationID)
				}
				if durability != -1 {
					item.SetDurability(durability)
				}
				item.IncrementVersion()
				itemsEncoded[j] = item.Encode(sess.GetUserId())
			}
		}
		return nil
	})
	i.JSON(&w, i.A{0, errorCodes, itemsEncoded})
}
