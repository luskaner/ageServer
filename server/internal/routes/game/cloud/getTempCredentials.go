package cloud

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetTempCredentials(w http.ResponseWriter, r *http.Request) {
	fullKey := r.URL.Query().Get("key")
	key := strings.TrimPrefix(fullKey, "/cloudfiles/")
	game := models.G(r)
	cloudfiles := game.Resources().CloudFiles()
	if cloudfiles == nil || !cloudfiles.Enabled() || cloudfiles.Credentials == nil {
		i.JSON(&w, i.A{2, 0, "", fullKey})
		return
	}
	// The credential is issued only once the key is known to exist: creating it
	// up front would leave a live session for credentialsExpiry on every call
	// with an unknown key.
	for file := range cloudfiles.Values() {
		if file.Key != key {
			continue
		}
		cred := models.CreateCredential(cloudfiles.Credentials, &key)
		expiry := cred.Expiry()
		i.JSON(&w, i.A{
			0,
			expiry.Unix(),
			"title=" + game.Title() + "&" + models.SasQuery(cred.Id(), expiry, file.Version, "r"),
			fullKey,
		})
		return
	}
	i.JSON(&w, i.A{2, models.CredentialExpiry(), "", fullKey})
}
