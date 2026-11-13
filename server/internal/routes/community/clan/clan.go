// Package clan implements the two public clan endpoints.
//
// The server has no clan model at all: models has no Clan type, the profiles
// table has no clan_id, and /game/clan/find is already a stub. getClanInfoFull
// would need ranks, per-rank permissions, member presence and moderation state,
// which is a whole subsystem rather than a read endpoint.
//
// Both routes are served anyway, with the correct outer shape and result.code 0,
// because a 404 makes a caller believe the server is broken while "no clan" is a
// state its client already handles (§0.3, §8.1).
package clan

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// FindResponse is the body of clan/find. Clans stays []any because there is no
// Clan type to point at yet; the array is always empty until one exists.
type FindResponse struct {
	Result shared.Result `json:"result"`
	Clans  []any         `json:"clans"`
}

// InfoResponse is the body of clan/getClanInfoFull. Clan is any for the same
// reason, and stays nil so it serialises as null.
type InfoResponse struct {
	Result shared.Result `json:"result"`
	Clan   any           `json:"clan"`
}

// Find returns an empty search result.
//
// TODO(clans): requires a clan model; see docs/community-api-implementation.md §8.1.
func Find(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, FindResponse{Result: shared.OK(), Clans: []any{}})
}

// GetClanInfoFull returns clan: null.
//
// null, not {}: the official service sends the full object or nothing, and an
// empty object would make the client read "members" off a clan that has none
// (§8.1).
//
// TODO(clans): requires a clan model; see docs/community-api-implementation.md §8.1.
func GetClanInfoFull(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, InfoResponse{Result: shared.OK(), Clan: nil})
}
