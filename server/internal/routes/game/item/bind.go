package item

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// bindAndResolveUser binds the request into req and resolves the session's user,
// writing errPayload on any failure. It is the prefix every item handler shares;
// errPayload is the handler's own error response, because the shape of it
// differs between endpoints.
func bindAndResolveUser[T any](w http.ResponseWriter, r *http.Request, req *T, errPayload i.A) (models.Game, models.User, bool) {
	if err := i.Bind(r, req); err != nil {
		i.JSON(&w, errPayload)
		return nil, nil, false
	}
	game := models.G(r)
	u, ok := game.Users().GetUserById(models.SessionOrPanic(r).GetUserId())
	if !ok {
		i.JSON(&w, errPayload)
		return nil, nil, false
	}
	return game, u, true
}
