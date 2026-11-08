package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// gamemodID is the game mode the official service reports on getMatchHistory.
// 1800 is the value seen on every captured AoE2 match (C15); the server does not
// record a mode of its own, so the constant is a documented deviation.
const gamemodID int32 = 1800

// GetMatchHistory returns the requested matches by id, plus the profiles that
// appear in them (the live service sends the profiles block here too, C5).
//
// This endpoint is not part of the OpenAPI spec the plan implements, but the
// official service answers it (confirmed 2026-11-08) and it is the only way to
// fetch a specific game, so it is provided as the optional phase 2.e.
func GetMatchHistory(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	var query shared.MatchQuery
	shared.BindQuery(r, &query)
	matchIDs := query.IDs()
	response := MatchHistoryResponse{
		Result:       shared.OK(),
		MatchHistory: make([]shared.Match, 0, len(matchIDs)),
		Profiles:     make([]*shared.Profile, 0),
	}
	seen := make(map[int32]struct{})
	for _, matchID := range matchIDs {
		match, found := game.MatchHistory().GetMatchWithPlayers(matchID)
		if !found {
			continue
		}
		response.MatchHistory = append(response.MatchHistory, encodeMatchObject(r, game, match, gamemodID))
		for _, player := range match.PlayerStats {
			seen[player.ProfileID] = struct{}{}
		}
	}
	// Resolve in one pass, through the same precedence the rest of the API uses.
	ids := make([]int32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	response.Profiles = shared.ResolveProfiles(r, shared.ProfileRequest{ProfileIDs: ids})
	if response.Profiles == nil {
		response.Profiles = []*shared.Profile{}
	}
	i.JSON(&w, response)
}
