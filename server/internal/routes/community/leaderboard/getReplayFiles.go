package leaderboard

import (
	"net/http"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// ReplayFilesResponse is the body of getReplayFiles.
type ReplayFilesResponse struct {
	Result      shared.Result       `json:"result"`
	ExpiryUnix  int64               `json:"expiryUnix"`
	ReplayFiles []shared.ReplayFile `json:"replayFiles"`
}

// GetReplayFiles returns the stored replays of the requested matches.
//
// This endpoint is not part of the OpenAPI spec either, but the official service
// answers it and it is how a tool fetches the replay of a specific game, so it is
// provided as the optional phase 2.f.
//
// Deviations from the official service, both documented:
//
//   - the URL is a plain LAN URL with no SAS signature, because this server does
//     not sign cloud files;
//   - size -1 is the official sentinel for "the replay was never uploaded", which
//     cannot happen here: a key only exists in the index once the file was
//     written.
func GetReplayFiles(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	var query shared.MatchQuery
	shared.BindQuery(r, &query)
	matchIDs := query.IDs()
	cloudFiles := game.Resources().CloudFiles()
	response := ReplayFilesResponse{
		Result:      shared.OK(),
		ExpiryUnix:  replayExpiryUnix(),
		ReplayFiles: make([]shared.ReplayFile, 0, len(matchIDs)),
	}
	if len(matchIDs) == 0 || cloudFiles == nil || !cloudFiles.ReplaysEnabled() {
		i.JSON(&w, response)
		return
	}
	for _, matchID := range matchIDs {
		for _, kind := range []int{0, 1} {
			key, entry, found := cloudFiles.FindReplay(matchID, kind)
			if !found {
				continue
			}
			response.ReplayFiles = append(response.ReplayFiles, shared.ReplayFile{
				// FIXME(replay-uploader): the uploader is not persisted, and this
				// endpoint is not scoped to a profile, so 0 is served.
				ProfileID:      0,
				MatchHistoryID: matchID,
				URL:            replayURL(r, key),
				Size:           int64(entry.Length),
				DataType:       kind,
			})
		}
	}
	i.JSON(&w, response)
}

// replayExpiryUnix is the moment the replay links stop being valid, or 0 when
// replay storage is disabled. The official service signs each URL with an
// expiry; this server has no signed URLs but still reports when its own files are
// swept, so a caller can refresh instead of keeping a dead link.
func replayExpiryUnix() int64 {
	if !i.ReplaysEnabled || i.ReplaysExpiry <= 0 {
		return 0
	}
	return time.Now().Add(i.ReplaysExpiry).Unix()
}
