package party

import (
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

type createOrReportSinglePlayerRequest struct {
	CreateMatchKey int32           `schema:"createMatchKey"`
	MatchKey       string          `schema:"matchKey"`
	MatchTypeID    int32           `schema:"matchTypeID"`
	MapName        string          `schema:"mapname"`
	IsComplete     int32           `schema:"isComplete"`
	AppBinCRC      int64           `schema:"appbincrc"`
	DataCRC        int64           `schema:"datacrc"`
	ModDLLCRC      int64           `schema:"moddllcrc"`
	ModDLLFile     string          `schema:"moddllfile"`
	ModName        string          `schema:"modname"`
	ModVersion     string          `schema:"modversion"`
	Options        string          `schema:"options"`  // zlib+base64, opaque
	SlotInfo       string          `schema:"slotInfo"` // zlib+base64, opaque
	CountersZip    string          `schema:"countersZip"`
	ItemUpdates    string          `schema:"itemUpdates"`
	Results        i.Json[[]int32] `schema:"results"`
	TeamIDs        i.Json[[]int32] `schema:"teamIDs"`
	RaceIDs        i.Json[[]int32] `schema:"race_ids"`
	XPGained       i.Json[[]int32] `schema:"xpGained"`
	VersionFlags   int32           `schema:"versionFlags"`
}

// singlePlayerMatchIDMu serializes generating and registering a match id. The
// database insert ignores collisions, so two concurrent requests could
// otherwise get the same id and attribute both results to one match.
var singlePlayerMatchIDMu sync.Mutex

// singlePlayerMatchIDAttempts bounds the collision retry. The clock advances
// every microsecond, so the first attempt almost always wins; the bound only
// stops a store that reports every id as taken from spinning with the mutex
// held.
const singlePlayerMatchIDAttempts = 100

func newSinglePlayerMatchID(history models.MatchHistory) int64 {
	singlePlayerMatchIDMu.Lock()
	defer singlePlayerMatchIDMu.Unlock()
	for attempt := 0; attempt < singlePlayerMatchIDAttempts; attempt++ {
		id := time.Now().UnixMicro() & 0x7fffffff
		if id == 0 || history.HasMatch(id) {
			continue
		}
		return id
	}
	// Every candidate collided: fall back to a random id in the same range
	// rather than blocking the request forever.
	var id int64
	i.WithRng(func(rand *i.RandReader) {
		id = int64(rand.Uint32() & 0x7fffffff)
	})
	if id == 0 {
		id = 1
	}
	return id
}

func newSinglePlayerMatchKey() string {
	b := make([]byte, 16)
	i.WithRng(func(rand *i.RandReader) {
		for idx := range b {
			b[idx] = byte(rand.UintN(256))
		}
	})
	return hex.EncodeToString(b)
}

// singlePlayerResponse is the 22-element positional response of
// createOrReportSinglePlayer.
func singlePlayerResponse(resultCode int, matchID int64, matchKey any, profile i.A, replays []i.A) i.A {
	slot13, slot14 := emptyReplayDescriptor(), emptyReplayDescriptor()
	if len(replays) > 0 {
		slot13 = replays[0]
	}
	if len(replays) > 1 {
		slot14 = replays[1]
	}
	var index12 any
	if len(replays) > 0 {
		index12 = ""
	}
	return i.A{
		resultCode,
		matchID,
		matchKey,
		profile,
		i.A{},
		i.A{},
		i.A{},
		nil,
		0,
		0,
		i.A{},
		i.A{},
		index12,
		slot13,
		slot14,
		i.A{},
		i.A{},
		i.A{},
		i.A{},
		i.A{},
		i.A{},
		i.A{},
	}
}

func sessionProfile(r *http.Request) (int32, i.A) {
	sess := models.SessionOrPanic(r)
	profileID := sess.GetUserId()
	profile := i.A{}
	if user, found := models.G(r).Users().GetUserById(profileID); found {
		profile = user.EncodeProfileInfo(sess.GetClientLibVersion())
	}
	return profileID, profile
}

func CreateOrReportSinglePlayer(w http.ResponseWriter, r *http.Request) {
	var req createOrReportSinglePlayerRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("party.createOrReportSinglePlayer: could not bind form: %s\n", err)
		i.JSON(&w, singlePlayerResponse(2, 0, nil, i.A{}, nil))
		return
	}
	profileID, profile := sessionProfile(r)
	history := models.G(r).MatchHistory()
	if history == nil {
		logger.Printf("party.createOrReportSinglePlayer: no match history store\n")
		i.JSON(&w, singlePlayerResponse(2, 0, nil, profile, nil))
		return
	}

	if req.CreateMatchKey == 1 {
		createSinglePlayerMatch(w, r, history, &req, profile)
		return
	}
	reportSinglePlayerMatch(w, r, history, &req, profileID, profile)
}

func createSinglePlayerMatch(w http.ResponseWriter, r *http.Request, history models.MatchHistory, req *createOrReportSinglePlayerRequest, profile i.A) {
	matchID := newSinglePlayerMatchID(history)
	matchKey := newSinglePlayerMatchKey()
	history.RecordPendingMatch(models.MatchResult{
		MatchID:   matchID,
		MapID:     req.MatchTypeID,
		MapName:   req.MapName,
		StartedAt: time.Now(),
	})
	if err := history.CreateSinglePlayerMatch(matchKey, matchID); err != nil {
		logger.Printf("party.createOrReportSinglePlayer: could not persist matchKey for match=%d: %s\n", matchID, err)
		i.JSON(&w, singlePlayerResponse(2, 0, nil, profile, nil))
		return
	}
	logger.Printf(
		"party.createOrReportSinglePlayer: created match=%d matchKey=%s map=%q matchTypeID=%d versionFlags=%d optionsLen=%d slotInfoLen=%d\n",
		matchID, matchKey, req.MapName, req.MatchTypeID, req.VersionFlags, len(req.Options), len(req.SlotInfo),
	)
	i.JSON(&w, singlePlayerResponse(0, matchID, matchKey, profile, nil))
}

func reportSinglePlayerMatch(w http.ResponseWriter, r *http.Request, history models.MatchHistory, req *createOrReportSinglePlayerRequest, profileID int32, profile i.A) {
	matchID, found := history.ResolveSinglePlayerMatch(req.MatchKey)
	if !found {
		logger.Printf("party.createOrReportSinglePlayer: unknown matchKey %q\n", req.MatchKey)
		i.JSON(&w, singlePlayerResponse(2, 0, nil, profile, nil))
		return
	}
	result := models.MatchResult{
		MatchID:    matchID,
		Players:    []int32{profileID},
		Results:    req.Results.Data,
		Teams:      req.TeamIDs.Data,
		Races:      req.RaceIDs.Data,
		XPGained:   req.XPGained.Data,
		WinnerTeam: -1,
		FinishedAt: time.Now(),
	}
	counters, err := decodeReportCounters(req.CountersZip, 1)
	if err != nil {
		logger.Printf("party.createOrReportSinglePlayer: match=%d invalid countersZip: %s\n", matchID, err)
	}
	result.Counters = counters
	var parsedUpdates i.Json[[]string]
	if parseErr := parsedUpdates.UnmarshalText([]byte(req.ItemUpdates)); parseErr != nil && strings.TrimSpace(req.ItemUpdates) != "" {
		logger.Printf("party.createOrReportSinglePlayer: match=%d invalid itemUpdates: %s\n", matchID, parseErr)
	}
	customUpdates, err := normalizeMatchCustomUpdates(parsedUpdates.Data, 1)
	if err != nil {
		logger.Printf("party.createOrReportSinglePlayer: match=%d invalid itemUpdates: %s\n", matchID, err)
	}
	result.CustomUpdates = customUpdates
	history.ApplyResult(result)

	var replays []i.A
	if req.IsComplete == 1 {
		replays = replayDescriptorsFor(r, matchID, singlePlayerReplayKinds(models.G(r).Title()), true)
	}
	logger.Printf(
		"party.createOrReportSinglePlayer: reported match=%d profile=%d isComplete=%d results=%v teams=%v races=%v optionsLen=%d slotInfoLen=%d replays=%d\n",
		matchID, profileID, req.IsComplete, result.Results, result.Teams, result.Races, len(req.Options), len(req.SlotInfo), len(replays),
	)
	i.JSON(&w, singlePlayerResponse(0, 0, nil, profile, replays))
}
