package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type findProfilesByPlatformIDRequest struct {
	PlatformIDs i.Json[[]uint64] `schema:"platformIDs"`
}

func FindProfilesByPlatformID(w http.ResponseWriter, r *http.Request) {
	var req findProfilesByPlatformIDRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	platformIdsMap := make(map[uint64]any, len(req.PlatformIDs.Data))
	for _, platformId := range req.PlatformIDs.Data {
		platformIdsMap[platformId] = struct{}{}
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	profileInfo := game.Users().EncodeProfileInfo(game.PresenceDefinitions(), func(currentUser models.User) bool {
		_, ok := platformIdsMap[currentUser.GetPlatformUserID()]
		return ok
	}, sess.GetClientLibVersion())
	i.JSON(&w, i.A{0, profileInfo})
}
