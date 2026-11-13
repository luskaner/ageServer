// Package communityEvent implements the public community-event endpoint.
package communityEvent

import (
	"encoding/json"
	"net/http"
	"time"

	commonGame "github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	athensEvent "github.com/luskaner/ageServer/server/internal/models/athens/routes/game/communityEvent"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
	gameCommunityEvent "github.com/luskaner/ageServer/server/internal/routes/game/communityEvent"
)

// Event is one community event.
//
// eventtype has no shared enum across titles: AoM uses 2, 3 and 5 while AoE4 uses
// 6, 0 and 7 (§5). localizedName is only present for the titles that publish one
// (AoE4 does, AoM does not).
type Event struct {
	ID                  uint64 `json:"id"`
	Name                string `json:"name"`
	Start               int64  `json:"start"`
	End                 int64  `json:"end"`
	ExpiryTime          int64  `json:"expiryTime"`
	ScheduledStartTime  int64  `json:"scheduledstarttime"`
	ScheduledEndTime    int64  `json:"scheduledendtime"`
	ScheduledExpiryTime int64  `json:"scheduledexpirytime"`
	ActualStartTime     int64  `json:"actualstarttime"`
	ActualEndTime       int64  `json:"actualendtime"`
	// CustomData travels verbatim as the JSON string the resource carries; it is
	// never re-serialised, because clients parse it a second time.
	CustomData    any    `json:"customdata"`
	EventState    int    `json:"eventstate"`
	EventType     int    `json:"eventtype"`
	LocalizedName string `json:"localizedName,omitempty"`
}

// Leaderboard is one leaderboards[] entry, only populated for AoM.
type Leaderboard struct {
	CommunityEventID uint64 `json:"communityevent_id"`
	ID               uint64 `json:"id"`
	Name             string `json:"name"`
	IsRanked         uint8  `json:"isranked"`
	ScoringType      int    `json:"scoringtype"`
}

// LeaderboardMap is one leaderboardMaps[] entry, only populated for AoM.
type LeaderboardMap struct {
	CommunityEventID            uint64 `json:"communityevent_id"`
	CommunityEventLeaderboardID uint64 `json:"communityeventleaderboard_id"`
	MatchTypeID                 int    `json:"matchtype_id"`
	StatGroupType               int    `json:"statgroup_type"`
	CivilizationID              int    `json:"civilization_id"`
}

// Response is the body of getAvailableCommunityEvents.
//
// Leaderboards and LeaderboardMaps are omitted when empty, which is what the live
// service does: age2 and age4 answer only communityEvents, while AoM sends the
// three blocks (C6).
type Response struct {
	Result          shared.Result    `json:"result"`
	CommunityEvents []Event          `json:"communityEvents"`
	Leaderboards    []Leaderboard    `json:"leaderboards,omitempty"`
	LeaderboardMaps []LeaderboardMap `json:"leaderboardMaps,omitempty"`
}

// GetAvailableCommunityEvents returns the community events of the title.
//
// The response shape depends on the title, which the live service confirms
// (C6, verified 2026-11-08):
//
//   - age2 and age4: only communityEvents, because the rest of the blocks the
//     game route computes are empty and the official service omits them;
//   - athens (AoM): communityEvents plus populated leaderboards and
//     leaderboardMaps. The AoM game model already computes those two blocks
//     (CommunityEventsEncoded), so this is a re-shaping, not new data work.
func GetAvailableCommunityEvents(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	response := Response{Result: shared.OK(), CommunityEvents: []Event{}}
	switch game.Title() {
	case commonGame.AoE4:
		// The AoE4 events are the rolling ranked season the game route builds.
		response.CommunityEvents = seasonEvents(game)
	case commonGame.AoM:
		events, leaderboards, leaderboardMaps := aomCommunityEvents()
		response.CommunityEvents = events
		response.Leaderboards = leaderboards
		response.LeaderboardMaps = leaderboardMaps
	}
	i.JSON(&w, response)
}

// seasonEvents re-shapes the AoE4 season events into objects.
//
// The game route builds them as
// [name, start, end, farFuture, id, eventtype, description, metadata]; the public
// API names the same values. eventtype is kept per title: AoE4 uses 6 for a
// ranked season, and the official AoM uses 2/3/5, so there is no shared enum
// (§5).
//
// The links/seasonLeaderboards blocks the game route also computes are not
// forwarded: on AoE4 their data already travels inside customdata, and the
// official public API does not repeat it.
func seasonEvents(game models.Game) []Event {
	blocks := gameCommunityEvent.SeasonEvents(game.Resources().ArrayFiles()["communityEvents.json"], time.Now())
	events := make([]Event, 0)
	// Slot 1 is the events table; the rest of the blocks are links and season
	// leaderboards.
	for _, raw := range shared.RowsAt(blocks, 1) {
		entry, ok := raw.(i.A)
		if !ok || len(entry) < 8 {
			continue
		}
		start, _ := shared.Int64At(entry, 1)
		end, _ := shared.Int64At(entry, 2)
		expiry, _ := shared.Int64At(entry, 3)
		id, _ := shared.Int64At(entry, 4)
		eventType, _ := shared.Int64At(entry, 5)
		events = append(events, Event{
			ID:                  uint64(id),
			Name:                shared.StringAt(entry, 0),
			Start:               start,
			End:                 end,
			ExpiryTime:          expiry,
			ScheduledStartTime:  start,
			ScheduledEndTime:    end,
			ScheduledExpiryTime: expiry,
			ActualStartTime:     start,
			ActualEndTime:       end,
			CustomData:          shared.At(entry, 7),
			EventState:          2,
			EventType:           int(eventType),
			// AoE4 publishes a localizedName; AoM does not (§5).
			LocalizedName: shared.StringAt(entry, 0),
		})
	}
	return events
}

// aomCommunityEvents re-shapes the typed AoM events of the current window.
//
// It reads the typed model values rather than the positional encoding the game
// client consumes: the encoding has seven blocks at fixed indices, and rebuilding
// objects from those slots would duplicate knowledge that already exists as
// fields. The events are generated per title, not per game instance, so no game
// argument is needed.
func aomCommunityEvents() ([]Event, []Leaderboard, []LeaderboardMap) {
	typed := athensEvent.Events()
	events := make([]Event, 0, len(typed))
	leaderboards := make([]Leaderboard, 0)
	leaderboardMaps := make([]LeaderboardMap, 0)
	for _, event := range typed {
		events = append(events, Event{
			ID:                  event.Id,
			Name:                event.Name,
			Start:               event.Start.Unix(),
			End:                 event.End.Unix(),
			ExpiryTime:          event.ExpiryTime.Unix(),
			ScheduledStartTime:  event.Start.Unix(),
			ScheduledEndTime:    event.End.Unix(),
			ScheduledExpiryTime: event.ExpiryTime.Unix(),
			ActualStartTime:     event.Start.Unix(),
			ActualEndTime:       event.End.Unix(),
			CustomData:          encodeCustomData(event.CustomData),
			EventState:          event.EventState,
			EventType:           event.EventType,
		})
		for _, leaderboard := range event.Leaderboards {
			leaderboards = append(leaderboards, Leaderboard{
				CommunityEventID: event.Id,
				ID:               leaderboard.Id,
				Name:             leaderboard.Name,
				IsRanked:         i.BoolToNumber(leaderboard.IsRanked),
				ScoringType:      leaderboard.ScoringType,
			})
			for _, mapping := range leaderboard.Maps {
				leaderboardMaps = append(leaderboardMaps, LeaderboardMap{
					CommunityEventID:            event.Id,
					CommunityEventLeaderboardID: leaderboard.Id,
					MatchTypeID:                 mapping.MatchtypeId,
					StatGroupType:               mapping.StatgroupType,
					CivilizationID:              mapping.CivilizationId,
				})
			}
		}
	}
	return events, leaderboards, leaderboardMaps
}

// encodeCustomData returns customdata as the string the API carries. A nil value
// stays nil, exactly like the game encoder; anything else is serialised as-is,
// without re-interpreting it.
func encodeCustomData(value any) any {
	if value == nil {
		return nil
	}
	if text, isText := value.(string); isText {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
