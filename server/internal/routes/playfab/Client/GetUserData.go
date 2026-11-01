package Client

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/models/playfab/data"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

func GetUserData(w http.ResponseWriter, r *http.Request) {
	sess := playfab.SessionOrPanic(r)
	shared.RespondOK(&w, getUserReadOnlyDataResponse{
		Data: map[string]any{
			"RLinkProfileID": data.NewBaseValue("public", sess.User().GetId()).ToValue(),
		},
	})
}
