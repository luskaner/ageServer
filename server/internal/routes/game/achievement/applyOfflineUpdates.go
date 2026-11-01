package achievement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func ApplyOfflineUpdates(w http.ResponseWriter, _ *http.Request) {
	// Which kind of updates?
	i.JSON(&w, i.A{0, i.A{}, i.A{}})
}
