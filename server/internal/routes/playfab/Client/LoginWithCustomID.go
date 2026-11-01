package Client

import (
	"net/http"
	"strconv"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type loginWithCustomIDResponse struct {
	loginWithSteamResponse
	infoResultPayload
}

type loginWithCustomIDRequest struct {
	CustomId string
}

func LoginWithCustomID(w http.ResponseWriter, r *http.Request) {
	if response := login(w, r, func(req loginWithCustomIDRequest, game playfab.Game) *playfab.SessionKey {
		userId, err := strconv.ParseInt(req.CustomId, 10, 32)
		if err != nil {
			shared.RespondBadRequest(&w)
			return nil
		}
		sessions := game.PlayfabSessions()
		return new(sessions.CreateWithUserId(game.Users(), int32(userId)))
	}); response != nil {
		shared.RespondOK(&w, loginWithCustomIDResponse{
			*response,
			infoResultPayload{
				UserInventory:        []any{},
				CharacterInventories: []any{},
			},
		})
	}
}
