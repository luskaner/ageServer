package communityEvent

import (
	"net/http"

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
	} else {
		response = i.A{0, i.A{}, i.A{}}
		if title == game.AoE2 || title == game.AoE4 {
			response = append(
				response,
				i.A{}, i.A{}, i.A{}, i.A{},
			)
		}
	}
	i.JSON(&w, response)
}
