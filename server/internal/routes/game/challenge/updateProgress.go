package challenge

import (
	"net/http"
)

func UpdateProgress(w http.ResponseWriter, r *http.Request) {
	updateProgress(w, r)
}
