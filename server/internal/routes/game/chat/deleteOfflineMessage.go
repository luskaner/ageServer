package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func DeleteOfflineMessage(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, i.A{0})
}
