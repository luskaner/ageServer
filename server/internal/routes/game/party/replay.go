package party

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

const cloudfilesVersion = "2019-02-02"

func emptyReplayDescriptor() i.A {
	return i.A{-1, "", "", "", 0, ""}
}

func replayDescriptor(matchID int64, url, host, path string, single bool, sig string) i.A {
	singleValue := 0
	if single {
		singleValue = 1
	}
	return i.A{matchID, url, host, path, singleValue, sig}
}

// replayKey derives the storage name of a match replay from (matchID, kind)
// without randomness, so a client retry overwrites the same file instead of
// accumulating duplicates and making the history nondeterministic.
func replayKey(matchID int64, kind int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d:%d", matchID, kind))
	return fmt.Sprintf("replays/%d/%d/%x.gz", matchID, kind, sum[:16])
}

// newReplayDescriptor issues an upload credential and builds the positional
// descriptor the client expects: [matchId, url, host, path, single, sas].
func newReplayDescriptor(r *http.Request, matchID int64, kind int, single bool) (i.A, bool) {
	store := models.G(r).Resources().CloudFiles()
	if store == nil || !store.ReplaysEnabled() {
		return emptyReplayDescriptor(), false
	}
	key := replayKey(matchID, kind)
	id, expiry, ok := store.Credential(key)
	if !ok {
		logger.Warn("party.replay: could not issue an upload credential for match=%d kind=%d", matchID, kind)
		return emptyReplayDescriptor(), false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	storagePath := "/cloudfiles/" + key
	return replayDescriptor(
		matchID,
		scheme+"://"+r.Host+storagePath,
		r.Host,
		storagePath,
		single,
		models.SasQuery(id, expiry, cloudfilesVersion, "cw"),
	), true
}

// reportMatchReplayKinds lists the artifacts a multiplayer report may expose.
// age1 and age3 have no replay slot; age4 exposes two; athens is assumed to
// match age2 until a multiplayer log is available.
func reportMatchReplayKinds(title string) []int {
	switch title {
	case game.AoE4:
		return []int{0, 1}
	case game.AoE2, game.AoM:
		return []int{0}
	default:
		return nil
	}
}

// singlePlayerReplayKinds lists the artifacts of a single-player match. age4
// and athens expose two, confirmed in the logs.
func singlePlayerReplayKinds(title string) []int {
	switch title {
	case game.AoE4, game.AoM:
		return []int{0, 1}
	default:
		return nil
	}
}

// replayDescriptorsFor builds one descriptor per kind, leaving the empty
// descriptor in place when the credential cannot be issued.
func replayDescriptorsFor(r *http.Request, matchID int64, kinds []int, single bool) []i.A {
	descriptors := make([]i.A, 0, len(kinds))
	for _, kind := range kinds {
		descriptor, ok := newReplayDescriptor(r, matchID, kind, single)
		if !ok {
			descriptors = append(descriptors, emptyReplayDescriptor())
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors
}
