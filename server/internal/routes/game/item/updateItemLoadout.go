package item

import (
	"net/http"

	mapset "github.com/deckarep/golang-set/v3"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type updateItemLoadoutRequest struct {
	itemIdLoadoutRequest
	createItemLoadoutRequest
}

func UpdateItemLoadout(w http.ResponseWriter, r *http.Request) {
	var req updateItemLoadoutRequest
	_, u, ok := bindAndResolveUser(w, r, &req, i.A{2, i.A{}})
	if !ok {
		return
	}
	var itemLoadoutEncoded i.A
	loadouts := u.GetItemLoadouts()
	if loadouts == nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	_ = loadouts.WithReadWrite(func(data *models.MainItemLoadouts) error {
		if item := data.Get(req.Id); item != nil {
			item.Update(req.Name, req.Type, mapset.NewThreadUnsafeSet(req.ItemOrLocIDs...))
			itemLoadoutEncoded = item.Encode(u.GetId())
		}
		return nil
	})
	if itemLoadoutEncoded == nil {
		i.JSON(&w, i.A{2, i.A{}})
	} else {
		i.JSON(&w, i.A{0, itemLoadoutEncoded})
	}
}
