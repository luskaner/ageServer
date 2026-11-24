package cacert_pem

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
)

// CacertPem serves the CA certificate of the listener's certificate mode:
// cacert.pem for modern and selfsigned_cert.pem for legacy. It is a route of the
// listener, not of a game, so the mode is a parameter; the leaf is never served
// here.
func CacertPem(w http.ResponseWriter, r *http.Request, mode common.CertMode) {
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

	file := common.CACert
	if mode == common.CertModeLegacy {
		file = common.SelfSignedCert
	}
	path := filepath.Join(folder, file)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.NotFound(w, r)
	} else {
		http.ServeFile(w, r, path)
	}
}
