package item

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type itemRequest struct {
	ItemIds i.Json[[]int32] `schema:"itemIDs"`
}

type moveItemRequest struct {
	itemRequest
	LocationIds i.Json[[]int32] `schema:"itemLocationIDs"`
	PositionIds i.Json[[]int32] `schema:"posIDs"`
	SlotIds     i.Json[[]int32] `schema:"slotIDs"`
}

func MoveItem(w http.ResponseWriter, r *http.Request) {
	var req moveItemRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	minLength := min(len(req.ItemIds.Data), len(req.LocationIds.Data), len(req.PositionIds.Data), len(req.SlotIds.Data))
	maxLength := max(len(req.ItemIds.Data), len(req.LocationIds.Data), len(req.PositionIds.Data), len(req.SlotIds.Data))
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
			positionID := req.PositionIds.Data[j]
			slotID := req.SlotIds.Data[j]
			if item, exists := (*data)[itemId]; !exists {
				errorCodes[j] = 2
				itemsEncoded[j] = i.A{}
			} else {
				errorCodes[j] = 0
				if locationID != -1 {
					item.SetLocationID(locationID)
				}
				if positionID != -1 {
					// TODO: Handle position if any game sends it
				}
				if slotID != -1 {
					item.GetMetadata().UpdateOther("eslot", strconv.FormatInt(int64(slotID), 10))
				}
				item.IncrementVersion()
				itemsEncoded[j] = item.Encode(sess.GetUserId())
			}
		}
		return nil
	})
	i.JSON(&w, i.A{0, errorCodes, itemsEncoded})
}
