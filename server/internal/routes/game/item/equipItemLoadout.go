package item

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type itemIdLoadoutRequest struct {
	Id int32 `schema:"id"`
}

func EquipItemLoadout(w http.ResponseWriter, r *http.Request) {
	// FIXME: What's the change after equip?
	var req itemIdLoadoutRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	userId := sess.GetUserId()
	u, ok := game.Users().GetUserById(userId)
	if !ok {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	var itemLoadoutEncoded i.A
	loadouts := u.GetItemLoadouts()
	if loadouts == nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	_ = loadouts.WithReadOnly(func(data *models.MainItemLoadouts) error {
		itemLoadout := data.Get(req.Id)
		if itemLoadout != nil {
			itemLoadoutEncoded = itemLoadout.Encode(userId)
		}
		return nil
	})
	if itemLoadoutEncoded == nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
	} else {
		i.JSON(&w, i.A{0, itemLoadoutEncoded, i.A{}})
	}
}
