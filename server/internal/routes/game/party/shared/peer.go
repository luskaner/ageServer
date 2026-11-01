package shared

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

type peerRequest struct {
	MatchID    int32           `schema:"match_id"`
	ProfileIDs i.Json[[]int32] `schema:"profile_ids"`
	RaceIDs    i.Json[[]int32] `schema:"race_ids"`
	TeamIDs    i.Json[[]int32] `schema:"teamIDs"`
}

func ParseParameters(r *http.Request) (parseError bool, advId int32, length int, profileIds []int32, raceIds []int32, teamIds []int32) {
	var req peerRequest
	err := i.Bind(r, &req)
	if err != nil {
		parseError = true
		return
	}
	profileIds = req.ProfileIDs.Data
	raceIds = req.RaceIDs.Data
	teamIds = req.TeamIDs.Data
	if min(len(profileIds), len(raceIds), len(teamIds)) != max(len(profileIds), len(raceIds), len(teamIds)) {
		parseError = true
		return
	}
	advId = req.MatchID
	length = len(profileIds)
	return
}
