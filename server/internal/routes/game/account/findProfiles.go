package account

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func FindProfiles(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.URL.Query().Get("name"))
	if len(name) < 1 {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	profileInfo := game.Users().EncodeProfileInfo(game.PresenceDefinitions(), func(currentUser models.User) bool {
		return strings.Contains(strings.ToLower(currentUser.GetAlias()), name)
	}, sess.GetClientLibVersion())
	i.JSON(&w, i.A{0, profileInfo})
}
