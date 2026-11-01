package clan

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func Create(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{2, nil, nil, i.A{}})
}
