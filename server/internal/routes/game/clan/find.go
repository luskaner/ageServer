package clan

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func Find(w http.ResponseWriter, _ *http.Request) {
	// FIXME: Try to avoid the client constinuously calling this endpoint like there were endless pages
	i.JSON(&w, i.A{0, i.A{}})
}
