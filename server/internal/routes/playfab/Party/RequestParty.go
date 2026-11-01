package Party

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

func RequestParty(w http.ResponseWriter, _ *http.Request) {
	shared.RespondNotAvailable(&w)
}
