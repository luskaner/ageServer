package clan

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

// FindClans returns an empty clan list.
// Public endpoint without a game session.
func FindClans(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{0, i.A{}})
}