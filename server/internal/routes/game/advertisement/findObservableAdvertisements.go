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
		// The official service only lists matches that have already started:
		// every advertisement captured from findObservableAdvertisements (2523 in
		// AoE4, 1602 in AoM, 76 in AoE3) had state=1 and a startTime, while
		// findAdvertisements only ever returned state=0 rows. Listing a lobby
		// that has not started yet also means reporting a null start time, which
		// is the timer the client draws for the match in issue #403.
		return advertisement.UnsafeGetObserversEnabled() && advertisement.UnsafeGetState() == 1
	}, true)
}
