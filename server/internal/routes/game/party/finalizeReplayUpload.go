package party

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

type finalizeReplayUploadRequest struct {
	MatchID        int64  `schema:"match_id"`
	URL            string `schema:"url"`
	Size           int64  `schema:"size"`
	FinalizeResult int32  `schema:"finalizeResult"`
	IsSinglePlayer int32  `schema:"isSinglePlayer"`
	ErrorString    string `schema:"errorString"`
}

// FinalizeReplayUpload always answers [0], like the official service. It only
// acts as a cleanup hook: a finalize that did not succeed discards the upload.
func FinalizeReplayUpload(w http.ResponseWriter, r *http.Request) {
	var req finalizeReplayUploadRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("party.finalizeReplayUpload: could not bind form: %s\n", err)
		i.JSON(&w, i.A{0})
		return
	}
	store := models.G(r).Resources().CloudFiles()
	index := strings.Index(req.URL, "/cloudfiles/")
	if index < 0 {
		logger.Warn("party.finalizeReplayUpload: match=%d url without /cloudfiles/: %q", req.MatchID, req.URL)
		i.JSON(&w, i.A{0})
		return
	}
	key := req.URL[index+len("/cloudfiles/"):]
	if query := strings.Index(key, "?"); query >= 0 {
		key = key[:query]
	}

	stored := false
	storedLength := 0
	if store != nil && store.Enabled() {
		if entry, ok := store.Get(key); ok {
			stored = true
			storedLength = entry.Length
		}
		if req.FinalizeResult != 1 {
			store.Delete(key)
		}
	}
	logger.Printf(
		"party.finalizeReplayUpload: match=%d size=%d finalizeResult=%d isSinglePlayer=%d errorString=%q key=%q stored=%t storedLength=%d\n",
		req.MatchID, req.Size, req.FinalizeResult, req.IsSinglePlayer, req.ErrorString, key, stored, storedLength,
	)
	i.JSON(&w, i.A{0})
}
