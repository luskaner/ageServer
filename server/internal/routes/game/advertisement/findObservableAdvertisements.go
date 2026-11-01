package advertisement

import (
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"

	"net/http"
)

func FindObservableAdvertisements(w http.ResponseWriter, r *http.Request) {
	var q wanQuery
	if err := i.Bind(r, &q); err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}}) // resultCode=2 (error), empty advertisements, empty user profiles
		return
	}
	g := models.G(r)
	title := g.Title()
	if title == game.AoE3 {
		observerGroupID := r.URL.Query().Get("observerGroupID")
		if observerGroupID != "0" {
			i.JSON(&w, i.A{0, i.A{}, i.A{}})
			return
		}
	}
	findAdvertisements(w, r, q.Length, q.Offset, true, nil, func(advertisement models.Advertisement) bool {
		return advertisement.UnsafeGetObserversEnabled()
	}, true)
}
