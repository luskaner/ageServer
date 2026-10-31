package challenge

import "net/http"

func UpdateProgressBatched(w http.ResponseWriter, r *http.Request) {
	updateProgress(w, r)
}
