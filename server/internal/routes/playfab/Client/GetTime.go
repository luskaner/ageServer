package Client

import (
	"net/http"
	"time"

	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type timeResponse struct {
	Time string
}

func GetTime(w http.ResponseWriter, _ *http.Request) {
	shared.RespondOK(
		&w,
		timeResponse{
			Time: shared.FormatDate(time.Now().UTC()),
		},
	)
}
