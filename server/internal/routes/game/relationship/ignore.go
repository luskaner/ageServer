package relationship

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func Ignore(w http.ResponseWriter, r *http.Request) {
	var req friendRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	game := models.G(r)
	u, ok := game.Users().GetUserById(req.TargetProfileID)
	if !ok {
		i.JSON(&w, i.A{2, i.A{}, i.A{}})
		return
	}
	i.JSON(&w, i.A{2, u.EncodeProfileInfo(models.SessionOrPanic(r).GetClientLibVersion()), i.A{}})
}
