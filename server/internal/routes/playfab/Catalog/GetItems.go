package Catalog

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type getItemsRequest struct {
	AlternateIds []any
	CustomTags   any
	Entity       any
	Ids          []string
}

type getItemsResponse struct {
	Items []playfab.CatalogItem
}

func GetItems(w http.ResponseWriter, r *http.Request) {
	var req getItemsRequest
	err := i.Bind(r, &req)
	if err != nil || len(req.Ids) == 0 {
		shared.RespondBadRequest(&w)
		return
	}
	genericGame := models.G(r)
	athensGame, ok := genericGame.(*athens.Game)
	if !ok {
		shared.RespondOK(&w, getItemsResponse{Items: []playfab.CatalogItem{}})
		return
	}
	game := athensGame
	catalogItemsMap := game.CatalogItems
	var catalogItems []playfab.CatalogItem
	for _, id := range req.Ids {
		catalogItem, ok := catalogItemsMap[id]
		if ok {
			catalogItems = append(catalogItems, catalogItem)
		}
	}
	shared.RespondOK(
		&w,
		getItemsResponse{
			Items: catalogItems,
		},
	)
}
