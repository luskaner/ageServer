package msstore

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func GetStoreTokens(w http.ResponseWriter, _ *http.Request) {
	// Likely just used to then send through platformlogin, is it for DLCs?
	i.JSON(&w, i.A{0, nil, ""})
}
