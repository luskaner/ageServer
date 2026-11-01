package Client

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type updateUserTitleDisplayNameRequest struct {
	DisplayName string
}

type updateUserTitleDisplayNameResponse struct {
	DisplayName string
}

func UpdateUserTitleDisplayName(w http.ResponseWriter, r *http.Request) {
	var req updateUserTitleDisplayNameRequest
	err := i.Bind(r, &req)
	if err != nil {
		shared.RespondBadRequest(&w)
		return
	}
	shared.RespondOK(
		&w,
		updateUserTitleDisplayNameResponse{
			DisplayName: req.DisplayName,
		},
	)
}
