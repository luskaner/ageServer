package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type getProfileNameRequest struct {
	ProfileIDs i.Json[[]int32] `schema:"profile_ids"`
}

func GetProfileName(w http.ResponseWriter, r *http.Request) {
	var req getProfileNameRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, i.A{}})
		return
	}
	profileIdsMap := make(map[int32]any, len(req.ProfileIDs.Data))
	for _, platformId := range req.ProfileIDs.Data {
		profileIdsMap[platformId] = struct{}{}
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	profileInfo := game.Users().EncodeProfileInfo(nil, func(currentUser models.User) bool {
		_, ok := profileIdsMap[currentUser.GetId()]
		return ok
	}, sess.GetClientLibVersion())
	i.JSON(&w, i.A{0, profileInfo})
}
