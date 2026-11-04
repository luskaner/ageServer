package clan

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

// GetClanInfoFull returns empty clan info.
// Public endpoint without a game session.
func GetClanInfoFull(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{0, i.A{}})
}