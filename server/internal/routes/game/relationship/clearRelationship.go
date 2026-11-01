package relationship

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func ClearRelationship(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{2})
}
