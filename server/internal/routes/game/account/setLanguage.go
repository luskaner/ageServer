package account

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func SetLanguage(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{2})
}
