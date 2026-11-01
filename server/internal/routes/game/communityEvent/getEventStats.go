package communityEvent

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func GetEventStats(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{2, i.A{}, i.A{}, i.A{}, i.A{}})
}
