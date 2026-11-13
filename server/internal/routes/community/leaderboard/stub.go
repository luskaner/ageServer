// Package leaderboard implements the public ladder, profile-stat and match
// history endpoints.
//
// Everything here works on objects and on the same stores the game routes use,
// but never through their positional encoders: the two layouts are different by
// design (§D1).
package leaderboard

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// GetStatGroupsByProfileIDs returns the GetPersonalStat shape with both arrays
// empty.
//
// The endpoint is listed in the wiki index but the live service answers 404
// (confirmed 2026-11-08). It is registered anyway so a caller written against the
// wiki gets a parseable answer; its payload mirrors GetPersonalStat because that
// is its documented equivalent (§8.4).
//
// NOTE: 404 in the official API; stub for robustness, see §8.4.
func GetStatGroupsByProfileIDs(w http.ResponseWriter, _ *http.Request) {
	i.JSON(&w, PersonalStatResponse{
		Result:           shared.OK(),
		StatGroups:       []shared.StatGroup{},
		LeaderboardStats: []shared.Stat{},
	})
}
