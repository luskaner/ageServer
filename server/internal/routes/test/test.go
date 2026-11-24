package test

import (
	"net/http"
	"strconv"

	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
)

// Test answers the announce probe for the game the listener resolved. The game
// and the certificate mode travel by parameter, not in the request context: this
// route is a route of the listener, which serves every enabled game, and the
// models.G type assertion would panic on the root routes where no game is fixed.
func Test(w http.ResponseWriter, r *http.Request, gameID string, mode common.CertMode) {
	data := i.AnnounceMessageData[gameID]
	data.Mode = string(mode)
	w.Header().Set(common.IdHeader, i.Id.String())
	w.Header().Set(common.VersionHeader, strconv.Itoa(common.AnnounceVersionLatest))
	i.JSON(&w, data)
}
