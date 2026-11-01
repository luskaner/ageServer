package item

import (
	"net/http"
	"slices"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type updateItemAttributesRequest struct {
	Keys    i.Json[[][]string] `json:"attributeKeys" schema:"attributeKeys"`
	Values  i.Json[[][]string] `json:"attributeValues" schema:"attributeValues"`
	ItemIds i.Json[[]int32]    `json:"itemInstance_ids" schema:"itemInstance_ids"`
	XpGains i.Json[[]int32]    `json:"xpGains" schema:"xpGains"`
}

func UpdateItemAttributes(w http.ResponseWriter, r *http.Request) {
	var req updateItemAttributesRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	minLength := min(len(req.Keys.Data), len(req.Values.Data), len(req.ItemIds.Data), len(req.XpGains.Data))
	maxLength := max(len(req.Keys.Data), len(req.Values.Data), len(req.ItemIds.Data), len(req.XpGains.Data))
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
	errorCodes := make([]i.A, minLength)
	itemsEncoded := make([]i.A, minLength)
	writeErr := u.GetItems().WithReadWrite(func(data *map[int32]*models.MainItem) error {
	outer:
		for j, itemId := range req.ItemIds.Data {
			var itemIdErrorCodes i.A
			attributeKeys := req.Keys.Data[j]
			attributeValues := req.Values.Data[j]
			if len(attributeKeys) != len(attributeValues) {
				itemIdErrorCodes = i.A{2, slices.Repeat(i.A{2}, max(len(attributeKeys), len(attributeValues)))}
				itemsEncoded[j] = i.A{}
				errorCodes[j] = itemIdErrorCodes
				continue
			}
			if item, exists := (*data)[itemId]; !exists {
				itemIdErrorCodes = i.A{2, slices.Repeat(i.A{2}, len(attributeKeys))}
				itemsEncoded[j] = i.A{}
			} else {
				metadata := item.GetMetadata()
				for k := range len(attributeKeys) {
					if !metadata.UpdateAttribute(attributeKeys[k], attributeValues[k]) {
						itemIdErrorCodes = i.A{2, slices.Repeat(i.A{2}, len(attributeKeys))}
						itemsEncoded[j] = i.A{}
						errorCodes[j] = itemIdErrorCodes
						continue outer
					}
				}
				item.IncrementVersion()
				itemIdErrorCodes = i.A{0, slices.Repeat(i.A{0}, len(attributeKeys))}
				itemsEncoded[j] = item.Encode(sess.GetUserId())
			}
			errorCodes[j] = itemIdErrorCodes
		}
		return nil
	})
	if writeErr != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	i.JSON(&w, i.A{0, errorCodes, itemsEncoded})
}
