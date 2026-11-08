package communityEvent

import (
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens"
)

func GetAvailableCommunityEvents(w http.ResponseWriter, r *http.Request) {
	var response i.A
	g := models.G(r)
	title := g.Title()
	if title == game.AoM {
		if ag, ok := g.(*athens.Game); ok {
			response = ag.CommunityEventsEncoded()
		} else {
			response = i.A{0, i.A{}, i.A{}}
		}
	} else if title == game.AoE4 {
		// The AoE4 client only enables the ranked queue while a type-6 ranked
		// season event covers the current time, so the LAN server publishes a
		// rolling season instead of an empty event list.
		response = aoe4CommunityEvents(g.Resources().ArrayFiles()["communityEvents.json"], time.Now())
	} else {
		response = i.A{0, i.A{}, i.A{}}
		if title == game.AoE2 {
			response = append(
				response,
				i.A{}, i.A{}, i.A{}, i.A{},
			)
		}
	}
	i.JSON(&w, response)
}
