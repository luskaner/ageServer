package party

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

// reportMatchRequest mirrors the form the client posts. The list fields travel
// as JSON arrays, which i.Json decodes transparently.
type reportMatchRequest struct {
	MatchID    int64           `schema:"match_id"`
	ProfileIDs i.Json[[]int32] `schema:"profile_ids"`
	Results    i.Json[[]int32] `schema:"results"`
	TeamIDs    i.Json[[]int32] `schema:"teamIDs"`
	RaceIDs    i.Json[[]int32] `schema:"race_ids"`
	XPGained   i.Json[[]int32] `schema:"xpGained"`
}

// reportMatchResponse is the payload the AoE2:DE client expects from
// /game/party/reportMatch. It has been verified against a real 1.16.0 match, so
// keep it byte-for-byte compatible.
func reportMatchResponse() i.A {
	return i.A{2, i.A{}, i.A{-1, "", "", "", 0, ""}, i.A{-1, "", "", "", 0, ""}, nil, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, 0, nil, i.A{}, i.A{}, i.A{}}
}

func ReportMatch(w http.ResponseWriter, r *http.Request) {
	var req reportMatchRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("party.reportMatch: could not bind form: %s\n", err)
		i.JSON(&w, reportMatchResponse())
		return
	}
	if req.MatchID == 0 {
		logger.Printf("party.reportMatch: missing or invalid match_id\n")
		i.JSON(&w, reportMatchResponse())
		return
	}

	result := models.MatchResult{
		MatchID:    req.MatchID,
		Players:    req.ProfileIDs.Data,
		Results:    req.Results.Data,
		Teams:      req.TeamIDs.Data,
		Races:      req.RaceIDs.Data,
		XPGained:   req.XPGained.Data,
		WinnerTeam: -1,
		FinishedAt: time.Now(),
	}
	if len(result.Players) == 0 {
		logger.Printf("party.reportMatch: match=%d has no profile_ids\n", req.MatchID)
		i.JSON(&w, reportMatchResponse())
		return
	}

	history := models.G(r).MatchHistory()
	if history == nil {
		logger.Printf("party.reportMatch: match=%d no match history store\n", req.MatchID)
		i.JSON(&w, reportMatchResponse())
		return
	}
	ratings, rated := history.ApplyResult(result)

	users := models.G(r).Users()
	var updates []string
	for _, profile := range result.Players {
		if rating, ok := ratings[profile]; ok && rating != nil {
			// Refresh the cached rating so the next profile encode shows the
			// new value without waiting for another database read.
			if user, found := users.GetUserById(profile); found {
				user.SetRating(rating.Rating)
			}
			updates = append(updates, strconv.FormatInt(int64(profile), 10)+"="+strconv.Itoa(int(rating.Rating)))
		}
	}
	logger.Printf(
		"party.reportMatch: stored match=%d profiles=%v results=%v teams=%v races=%v rated=%t ratings=[%s]\n",
		req.MatchID,
		result.Players,
		result.Results,
		result.Teams,
		result.Races,
		rated,
		strings.Join(updates, " "),
	)
	i.JSON(&w, reportMatchResponse())
}
