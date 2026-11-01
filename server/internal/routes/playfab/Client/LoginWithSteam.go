package Client

import (
	"io"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/uuid"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type entityResponse struct {
	Id         string
	Type       string
	TypeString string
}

type treatmentAssignmentResponse struct {
	Variants  []any
	Variables []any
}

type entityTokenResponse struct {
	EntityToken     string
	TokenExpiration string
	entityResponse  `json:"Entity"`
}

type settingsForUserResponse struct {
	NeedsAttribution bool
	GatherDeviceInfo bool
	GatherFocusInfo  bool
}

type loginWithSteamRequest struct {
	SteamTicket string
}

type loginWithSteamResponse struct {
	SessionTicket               string
	PlayFabId                   string
	NewlyCreated                bool
	settingsForUserResponse     `json:"SettingsForUser"`
	LastLoginTime               string
	entityTokenResponse         `json:"EntityToken"`
	treatmentAssignmentResponse `json:"TreatmentAssignment"`
}

func login[R any](w http.ResponseWriter, r *http.Request, reqToPlayfabID func(req R, game playfab.Game) *playfab.SessionKey) *loginWithSteamResponse {
	var req R
	err := i.Bind(r, &req)
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)
	if err != nil {
		shared.RespondBadRequest(&w)
		return nil
	}
	now := time.Now().UTC()
	genericG := models.G(r)
	playfabGame, ok := genericG.(playfab.Game)
	if !ok {
		shared.RespondBadRequest(&w)
		return nil
	}
	id := reqToPlayfabID(req, playfabGame)
	if id == nil {
		shared.RespondBadRequest(&w)
		return nil
	}
	return &loginWithSteamResponse{
		SessionTicket:    *id,
		PlayFabId:        *id,
		NewlyCreated:     true,
		NeedsAttribution: false,
		GatherDeviceInfo: true,
		GatherFocusInfo:  true,
		LastLoginTime:    shared.FormatDate(time.Date(2025, 11, 12, 3, 34, 0, 0, time.UTC)),
		EntityToken:      *id,
		TokenExpiration:  shared.FormatDate(now.AddDate(0, 0, 1)),
		Id:               uuid.New().String(),
		Type:             "title_player_account",
		TypeString:       "title_player_account",
		Variants:         []any{},
		Variables:        []any{},
	}
}

func LoginWithSteam(w http.ResponseWriter, r *http.Request) {
	if response := login(w, r, func(req loginWithSteamRequest, game playfab.Game) *playfab.SessionKey {
		steamId, err := playfab.ParseSteamIDHex(req.SteamTicket)
		if err != nil {
			shared.RespondBadRequest(&w)
			return nil
		}
		sessions := game.PlayfabSessions()
		return new(sessions.CreateWithSteamUserId(game.Users(), steamId))
	}); response != nil {
		shared.RespondOK(&w, response)
	}
}
