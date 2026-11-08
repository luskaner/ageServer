package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/game/leaderboard/shared"
)

// GetPersonalStat returns the five positional blocks captured from the
// official service for AoE2/AoE3/AoE4/AoM:
//
//	[0, [statGroup...], [profileInfo...], [leaderboardStat...], [personalStat...]]
//
// The fifth block is the avatar/personal statistics row of each profile:
// [statID, profileID, value, metadata, timestamp]. See docs/matchmaking.md
// §2.6 (D10).
func GetPersonalStat(w http.ResponseWriter, r *http.Request) {
	i.JSON(&w, shared.GetPersonalStat(r, r.URL.Query().Get("profileids")))
}
