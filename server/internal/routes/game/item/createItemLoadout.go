package item

import (
	"net/http"

	mapset "github.com/deckarep/golang-set/v3"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type createItemLoadoutRequest struct {
	ItemOrLocIDs []int32 `schema:"itemOrLocIDs"`
	Name         string  `schema:"name"`
	Type         int32   `schema:"type"`
	// TODO: Implement fields
	// AttributeKeys  []any `schema:"attributeKeys"`
	// RecurseLevels []int32 `schema:"recurseLevels"`
}

func CreateItemLoadout(w http.ResponseWriter, r *http.Request) {
	var req createItemLoadoutRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	userId := sess.GetUserId()
	u, ok := game.Users().GetUserById(userId)
	if !ok {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	items := game.Items()
	itemIds := mapset.NewThreadUnsafeSet[int32]()
	for _, id := range req.ItemOrLocIDs {
		if _, ok = items.GetLocation(id); !ok {
			itemIds.Add(id)
		}
	}
	if !itemIds.IsEmpty() {
		_ = u.GetItems().WithReadOnly(func(data *map[int32]*models.MainItem) error {
			ok = itemIds.IsSubset(mapset.NewThreadUnsafeSetFromMapKeys(*data))
			return nil
		})
	} else {
		ok = true
	}
	if !ok {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	var itemLoadoutEncoded i.A
	loadouts := u.GetItemLoadouts()
	if loadouts == nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	_ = loadouts.WithReadWrite(func(data *models.MainItemLoadouts) error {
		itemLoadoutEncoded = data.NewItemLoadout(req.Name, req.Type, mapset.NewThreadUnsafeSet(req.ItemOrLocIDs...), userId)
		return nil
	})
	i.JSON(&w, i.A{0, itemLoadoutEncoded})
}
