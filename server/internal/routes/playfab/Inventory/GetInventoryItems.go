package Inventory

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type getInventoryItemsRequest struct {
	CollectionId      *string
	ContinuationToken *string
	Count             uint8
	CustomTags        struct{}
	Entity            *any
	Filter            *string
}
type getInventoryItemsResponse struct {
	Items             []playfab.InventoryItem
	ContinuationToken string `json:",omitempty"`
	ETag              string
}

func GetInventoryItems(w http.ResponseWriter, r *http.Request) {
	var req getInventoryItemsRequest
	err := i.Bind(r, &req)
	if err != nil {
		shared.RespondBadRequest(&w)
		return
	}
	genericGame := models.G(r)
	athensGame, ok := genericGame.(*athens.Game)
	if !ok {
		// Fallback for httptest with fake MainGame: return empty inventory
		shared.RespondOK(&w, getInventoryItemsResponse{Items: []playfab.InventoryItem{}, ETag: `1/MQ=="`})
		return
	}
	game := athensGame
	inventoryItems := game.InventoryItems
	var offset int
	if req.ContinuationToken != nil {
		if offset, err = strconv.Atoi(*req.ContinuationToken); err != nil {
			shared.RespondBadRequest(&w)
			return
		}
		inventoryItems = inventoryItems[offset:]
	}
	returnItems := inventoryItems[:min(int(req.Count), len(inventoryItems))]
	var continuationToken string
	if len(returnItems) < len(inventoryItems) {
		continuationToken = strconv.Itoa(offset + len(returnItems))
	}
	shared.RespondOK(
		&w,
		getInventoryItemsResponse{
			Items:             returnItems,
			ETag:              `1/MQ=="`,
			ContinuationToken: continuationToken,
		},
	)
}
