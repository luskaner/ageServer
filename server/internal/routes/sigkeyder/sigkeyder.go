package sigkeyder

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
)

func SigkeyDer(w http.ResponseWriter, r *http.Request) {
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

	path := filepath.Join(folder, "sigkey.der")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.NotFound(w, r)
	} else {
		http.ServeFile(w, r, path)
	}
}