package party

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

// reportMatchRequest mirrors the form the client posts. The list fields travel
// as JSON arrays, which i.Json decodes transparently. ItemUpdates is kept as
// raw text so one malformed value cannot fail the whole bind.
type reportMatchRequest struct {
	MatchID      int64           `schema:"match_id"`
	ProfileIDs   i.Json[[]int32] `schema:"profile_ids"`
	Results      i.Json[[]int32] `schema:"results"`
	TeamIDs      i.Json[[]int32] `schema:"teamIDs"`
	RaceIDs      i.Json[[]int32] `schema:"race_ids"`
	XPGained     i.Json[[]int32] `schema:"xpGained"`
	CountersZip  string          `schema:"countersZip"`
	ItemUpdates  string          `schema:"itemUpdates"`
	// simplayerIDs is a client-side unsigned value: AoE III sends 4294967295
	// (0xFFFFFFFF, "no sim player") and int32 cannot hold it, which failed the
	// whole bind and dropped the match. It is only logged, so int64 is enough.
	SimPlayerIDs i.Json[[]int64] `schema:"simplayerIDs"`
	CheckSums    i.Json[[]int32] `schema:"checkSums"`
	LastCallTime int64           `schema:"lastCallTime"`
}

const maxReportCountersSize = 1 << 20

const (
	rankedAutomatchQueueID int32 = 6
	rankedAutomatchTypeID  int32 = 4
)

// defaultMatchCustomUpdate holds the empty document used for a key the client
// did not send. Unknown keys are kept as received so client-side data such as
// customScoring survives a round trip through the server.
var defaultMatchCustomUpdate = map[string]string{
	"itemUpdates":           "[]",
	"challengeUpdates":      "[]",
	"eventChallengeUpdates": "[]",
	"clientUpdates":         "{}",
}

// reportMatchResponse matches the official positional response. A successful
// report uses result code 0; malformed or unprocessable reports use code 2.
// replays holds the descriptors for slots 2 and 3, in that order. The response
// always has 16 elements, including age1: see replay-implementation.md §7.2.
func reportMatchResponse(resultCode int, replays []i.A) i.A {
	slot2, slot3 := emptyReplayDescriptor(), emptyReplayDescriptor()
	if len(replays) > 0 {
		slot2 = replays[0]
	}
	if len(replays) > 1 {
		slot3 = replays[1]
	}
	return i.A{resultCode, i.A{}, slot2, slot3, nil, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, 0, nil, i.A{}, i.A{}, i.A{}}
}

func decodeReportCounters(encoded string, playerCount int) ([]string, error) {
	// An empty value means "keep whatever is stored": the SQL upsert only
	// replaces a column when the new value is not empty.
	counters := make([]string, playerCount)
	if strings.TrimSpace(encoded) == "" {
		return counters, nil
	}

	var compressedValue string
	if err := json.Unmarshal([]byte(encoded), &compressedValue); err != nil {
		return counters, err
	}
	compressed, err := base64.StdEncoding.DecodeString(compressedValue)
	if err != nil {
		return counters, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return counters, err
	}
	payload, readErr := io.ReadAll(io.LimitReader(reader, maxReportCountersSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return counters, readErr
	}
	if closeErr != nil {
		return counters, closeErr
	}
	if len(payload) > maxReportCountersSize {
		return counters, errors.New("decompressed counters exceed the size limit")
	}

	var decoded []string
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return counters, err
	}
	if len(decoded) != playerCount {
		return counters, errors.New("counter count does not match profile_ids")
	}
	return decoded, nil
}

func normalizeMatchCustomUpdates(updates []string, playerCount int) ([]string, error) {
	// An empty value means "keep whatever is stored".
	normalized := make([]string, playerCount)
	if len(updates) == 0 {
		return normalized, nil
	}
	if len(updates) != playerCount {
		return normalized, errors.New("itemUpdates count does not match profile_ids")
	}

	var firstErr error
	for idx, raw := range updates {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if decoded == nil {
			decoded = make(map[string]json.RawMessage)
		}
		for key, value := range defaultMatchCustomUpdate {
			if len(decoded[key]) == 0 {
				decoded[key] = json.RawMessage(value)
			}
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		normalized[idx] = string(encoded)
	}
	return normalized, firstErr
}

func ReportMatch(w http.ResponseWriter, r *http.Request) {
	var req reportMatchRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("party.reportMatch: could not bind form: %s\n", err)
		i.JSON(&w, reportMatchResponse(2, nil))
		return
	}
	if req.MatchID == 0 {
		logger.Printf("party.reportMatch: missing or invalid match_id\n")
		i.JSON(&w, reportMatchResponse(2, nil))
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
		i.JSON(&w, reportMatchResponse(2, nil))
		return
	}
	counters, err := decodeReportCounters(req.CountersZip, len(result.Players))
	if err != nil {
		logger.Printf("party.reportMatch: match=%d invalid countersZip: %s\n", req.MatchID, err)
	}
	result.Counters = counters
	var parsedUpdates i.Json[[]string]
	if parseErr := parsedUpdates.UnmarshalText([]byte(req.ItemUpdates)); parseErr != nil && strings.TrimSpace(req.ItemUpdates) != "" {
		// A malformed itemUpdates must not prevent the match from being stored.
		logger.Printf("party.reportMatch: match=%d invalid itemUpdates, using defaults: %s\n", req.MatchID, parseErr)
	}
	customUpdates, err := normalizeMatchCustomUpdates(parsedUpdates.Data, len(result.Players))
	if err != nil {
		logger.Printf("party.reportMatch: match=%d invalid itemUpdates: %s\n", req.MatchID, err)
	}
	result.CustomUpdates = customUpdates

	history := models.G(r).MatchHistory()
	if history == nil {
		logger.Printf("party.reportMatch: match=%d no match history store\n", req.MatchID)
		i.JSON(&w, reportMatchResponse(2, nil))
		return
	}
	ratings, rated := history.ApplyResult(result)
	if !history.HasMatch(req.MatchID) {
		logger.Printf("party.reportMatch: match=%d was not persisted\n", req.MatchID)
		i.JSON(&w, reportMatchResponse(2, nil))
		return
	}

	users := models.G(r).Users()
	storedProfiles := history.GetProfiles(result.Players)
	var updates []string
	statGroupIDs := make(map[int32]int32, len(result.Players))
	for _, profile := range result.Players {
		var userStatGroupID, storedStatGroupID, ratingStatGroupID int32
		if user, found := users.GetUserById(profile); found {
			userStatGroupID = user.GetStatGroupID()
		}
		if stored, ok := storedProfiles[profile]; ok && stored != nil {
			storedStatGroupID = stored.StatGroupID
		}
		rating := ratings[profile]
		if rating != nil {
			ratingStatGroupID = rating.StatID
		}
		statGroupIDs[profile] = models.ResolveStatGroupID(userStatGroupID, storedStatGroupID, ratingStatGroupID)
		if rating != nil {
			// Refresh the cached rating so the next profile encode shows the
			// new value without waiting for another database read.
			if user, found := users.GetUserById(profile); found {
				user.SetRating(rating.Rating)
			}
			updates = append(updates, strconv.FormatInt(int64(profile), 10)+"="+strconv.Itoa(int(rating.Rating)))
		}
	}
	logger.Printf(
		"party.reportMatch: stored match=%d profiles=%v results=%v teams=%v races=%v simplayerIDs=%v checkSums=%v lastCallTime=%d rated=%t ratings=[%s]\n",
		req.MatchID,
		result.Players,
		result.Results,
		result.Teams,
		result.Races,
		req.SimPlayerIDs.Data,
		req.CheckSums.Data,
		req.LastCallTime,
		rated,
		strings.Join(updates, " "),
	)
	if rated {
		notification := gameResultNotification(result, ratings, statGroupIDs)
		for _, profile := range result.Players {
			if session, found := models.G(r).Sessions().GetByUserId(profile); found {
				wss.SendOrStoreMessage(session, "GameResultNotificationMessage", notification)
			}
		}
	}
	replays := replayDescriptorsFor(r, req.MatchID, reportMatchReplayKinds(models.G(r).Title()), false)
	i.JSON(&w, reportMatchResponse(0, replays))
}

func gameResultNotification(match models.MatchResult, ratings map[int32]*models.PlayerRating, statGroupIDs map[int32]int32) i.A {
	results := make(i.A, 0, len(match.Players))
	leaderboardStats := make(i.A, 0, len(match.Players))
	for idx, profileID := range match.Players {
		result := int32(-1)
		if idx < len(match.Results) {
			result = match.Results[idx]
		}
		statGroupID := statGroupIDs[profileID]
		results = append(results, i.A{
			profileID,
			rankedAutomatchQueueID,
			rankedAutomatchTypeID,
			result,
			i.A{0},
			statGroupID,
			i.A{},
			0,
			0,
			i.A{},
			nil,
			i.A{},
			i.A{},
			i.A{},
			i.A{},
			i.A{},
			i.A{},
		})

		rating := ratings[profileID]
		if rating == nil {
			rating = &models.PlayerRating{
				ProfileID:     profileID,
				LeaderboardID: models.SoloRMRanked,
				Rating:        models.DefaultRating,
				HighestRating: models.DefaultRating,
			}
		}
		leaderboardStats = append(
			leaderboardStats,
			models.EncodeLeaderboardStatFor(statGroupID, rating.Rating, rating),
		)
	}

	return i.A{results, leaderboardStats, i.A{}, match.MatchID, "", i.A{}}
}
