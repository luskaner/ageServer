// Package item implements the public inventory endpoints.
package item

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// ItemsResponse is the body of getItems. Items stays []any because the endpoint
// has no documented item shape: it is a stub for robustness only (§8.4).
type ItemsResponse struct {
	Result shared.Result `json:"result"`
	Items  []any         `json:"items"`
}

// InventoryResponse is the body of getInventoryByProfileIDs.
//
// The map is one of the few responses that stays map-based: the keys are the
// caller-supplied profile ids wrapped in literal quotes, which cannot be struct
// fields (§3.b).
type InventoryResponse struct {
	Result                   shared.Result      `json:"result"`
	ItemInstancesByProfileID []map[string][]any `json:"itemInstancesByProfileID"`
}

// GetItems returns an empty item list.
//
// The endpoint figures in the wiki index but the live service answers 404
// (confirmed 2026-11-08). It is registered so a client written against the wiki
// gets a parseable answer instead of an opaque 404 (§8.4).
//
// NOTE: 404 in the official API; stub for robustness, see §8.4.
func GetItems(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, ItemsResponse{Result: shared.OK(), Items: []any{}})
}

// GetInventoryByProfileIDs returns one empty inventory per requested profile.
//
// Two oddities are replicated on purpose because they are the official
// behaviour, not a gap of this server (§3.b):
//
//  1. the map key contains literal double quotes ({"\"196240\"":[]}),
//  2. the instance array is always empty here; the real inventory is only
//     served to the game client through /game.
//
// The number of entries does follow the request: a caller asking for three
// profiles gets three keys, which is the part of the shape a client actually
// relies on.
func GetInventoryByProfileIDs(w http.ResponseWriter, r *http.Request) {
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	ids := query.ProfileRequest().ProfileIDs
	response := InventoryResponse{
		Result:                   shared.OK(),
		ItemInstancesByProfileID: make([]map[string][]any, 0, len(ids)),
	}
	for _, id := range ids {
		// The quotes are part of the key the official service sends.
		key := `"` + strconv.FormatInt(int64(id), 10) + `"`
		response.ItemInstancesByProfileID = append(response.ItemInstancesByProfileID,
			map[string][]any{key: {}})
	}
	i.JSON(&w, response)
}
