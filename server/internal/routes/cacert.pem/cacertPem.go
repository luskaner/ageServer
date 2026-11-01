package cacert_pem

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal/models"
)

func CacertPem(w http.ResponseWriter, r *http.Request) {
	exe, err := os.Executable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	folder := common.CertificatePairFolder(exe)
	if folder == "" {
		http.NotFound(w, r)
		return
	}

	var file string
	if common.SelfSignedCertGame(models.G(r).Title()) {
		file = common.SelfSignedCert
	} else {
		file = common.CACert
	}
	path := filepath.Join(folder, file)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.NotFound(w, r)
	} else {
		http.ServeFile(w, r, path)
	}
}
