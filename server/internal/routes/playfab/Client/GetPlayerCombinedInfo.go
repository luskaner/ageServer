package Client

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type userReadonlyData struct{}

type infoResultPayload struct {
	UserInventory           []any
	UserDataVersion         int
	userReadonlyData        `json:"UserReadOnlyData"`
	UserReadOnlyDataVersion int
	CharacterInventories    []any
}
type getPlayerCombinedInfoRequest struct {
	PlayFabId         string
	infoResultPayload `json:"InfoResultPayload"`
}

func GetPlayerCombinedInfo(w http.ResponseWriter, r *http.Request) {
	sess := playfab.SessionOrPanic(r)
	shared.RespondOK(
		&w,
		getPlayerCombinedInfoRequest{
			PlayFabId:            sess.PlayfabId(),
			UserInventory:        []any{},
			CharacterInventories: []any{},
		},
	)
}
