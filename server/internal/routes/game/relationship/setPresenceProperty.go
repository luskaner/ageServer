package relationship

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type setPresencePropertyRequest struct {
	PresencePropertyId int32  `schema:"presencePropertyDef_id"`
	Value              string `schema:"value"`
}

func SetPresenceProperty(w http.ResponseWriter, r *http.Request) {
	var req setPresencePropertyRequest
	if err := i.Bind(r, &req); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	sess := models.SessionOrPanic(r)
	game := models.G(r)
	users := game.Users()
	u, _ := users.GetUserById(sess.GetUserId())
	u.SetPresenceProperty(req.PresencePropertyId, req.Value)
	NotifyChangePresence(sess.GetClientLibVersion(), game.Sessions(), users, u, game.PresenceDefinitions())
	i.JSON(&w, i.A{0})
}
