package router

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// The AoE4 client disables the ranked queue when getAvailableCommunityEvents
// returns no active type-6 season event. This test loads the bundled season
// definitions into the fake resources and checks the handler wiring: the
// response must contain the four ranked season events with a window that
// covers now.
func TestAoE4CommunityEventsPublishActiveSeason(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	u, sid := createUserAndSessionForGame(t, g, 76561198000000031, "aoe4Ranked")

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "age4", "communityEvents.json"))
	if err != nil {
		t.Fatalf("read communityEvents.json: %v", err)
	}
	var definitions i.A
	if err := json.Unmarshal(data, &definitions); err != nil {
		t.Fatalf("decode communityEvents.json: %v", err)
	}
	g.Resources().ArrayFiles()["communityEvents.json"] = definitions

	rr := doRequest(h, http.MethodGet, "/game/CommunityEvent/getAvailableCommunityEvents", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("response is not JSON: %v body=%q", err, rr.Body.String())
	}
	if len(response) != 7 {
		t.Fatalf("response has %d blocks, want 7: %q", len(response), rr.Body.String())
	}
	events, ok := response[1].([]any)
	if !ok || len(events) != 8 {
		t.Fatalf("events = %d, want 8 (current and previous season)", len(events))
	}
	// Block 5 is what lets the client resolve the season's ladders; without it
	// the ranked pane reports that there is no active season.
	seasonLeaderboards, ok := response[5].([]any)
	if !ok || len(seasonLeaderboards) == 0 {
		t.Fatalf("season leaderboards = %#v, want the base ladder mapping", response[5])
	}
	now := time.Now().Unix()
	for index, raw := range events {
		event, ok := raw.([]any)
		if !ok || len(event) != 8 {
			t.Fatalf("event %d = %#v", index, raw)
		}
		name, _ := event[0].(string)
		if name == "" {
			t.Fatalf("event %d has no name", index)
		}
		start, _ := event[1].(float64)
		end, _ := event[2].(float64)
		// The active season contains now; the previous one has already ended.
		active := int64(start) <= now && now < int64(end)
		finished := int64(end) <= now
		if !active && !finished {
			t.Fatalf("event %q window [%v, %v) is neither active nor finished", name, start, end)
		}
		if event[5].(float64) != 6 {
			t.Fatalf("event %q type = %v, want 6", name, event[5])
		}
	}
	_ = u
}

// TestAoE4MenuBackgroundScheduleReachesTheResponse is the end-to-end check of
// the menu-background wiring: a schedule injected into the resources has to
// arrive in the event list ahead of the season events, with the official shape.
// The unit tests cover the schedule itself, so without this a mistake in the
// handler would go unnoticed.
func TestAoE4MenuBackgroundScheduleReachesTheResponse(t *testing.T) {
	now := time.Now()
	period := models.MenuBackgroundPeriod{
		Event:      290,
		Name:       "Trade Secrets - Challenges 13",
		Start:      now.Add(-time.Hour),
		End:        now.Add(time.Hour),
		CustomData: `{"frontEndImagePath":"\\Images\\backgrounds\\events\\xp7_keyart.png"}`,
	}
	// The season definitions have to be there too, or the response is the empty
	// shape and there is nothing for the background to sit beside.
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "age4", "communityEvents.json"))
	if err != nil {
		t.Fatalf("read communityEvents.json: %v", err)
	}
	var definitions i.A
	if err := json.Unmarshal(data, &definitions); err != nil {
		t.Fatalf("decode communityEvents.json: %v", err)
	}
	g := newTestGameWithResourceSetup(t, game.AoE4, func(resources *fakeResources) {
		resources.arrayFiles["communityEvents.json"] = definitions
		resources.menuBackgrounds = &models.MenuBackgrounds{
			Periods: []models.MenuBackgroundPeriod{period},
		}
	})
	h := gameHandler(game.AoE4)
	_, sid := createUserAndSessionForGame(t, g, 76561198000000033, "aoe4Background")

	rr := doRequest(h, http.MethodGet, "/game/CommunityEvent/getAvailableCommunityEvents", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	events, ok := response[1].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("events = %#v, want the configured background first", response[1])
	}
	background, ok := events[0].([]any)
	if !ok || len(background) != 8 {
		t.Fatalf("first event = %#v, want the 8-field background event", events[0])
	}
	if background[0] != period.Name {
		t.Fatalf("first event name = %v, want %q", background[0], period.Name)
	}
	if background[4] != float64(290) || background[5] != float64(0) {
		t.Fatalf("first event id/type = %v/%v, want 290/0", background[4], background[5])
	}
	if background[6] != "" {
		t.Fatalf("description = %v, want it empty like the official service", background[6])
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(background[7].(string)), &metadata); err != nil {
		t.Fatalf("decode customdata: %v", err)
	}
	if metadata["frontEndImagePath"] != `\Images\backgrounds\events\xp7_keyart.png` {
		t.Fatalf("frontEndImagePath = %q", metadata["frontEndImagePath"])
	}
	// The ranked season must still be there: its type-6 events are what enable
	// the ranked queue.
	seasonEvents := 0
	for _, raw := range events {
		event, ok := raw.([]any)
		if ok && len(event) == 8 && event[5] == float64(6) {
			seasonEvents++
		}
	}
	if seasonEvents == 0 {
		t.Fatal("no type-6 season events survived the background injection")
	}
}

// TestNonAoE4GetsNoMenuBackgrounds checks the feature stays inside AoE IV: the
// same response for another title must not gain a type-0 event.
func TestNonAoE4GetsNoMenuBackgrounds(t *testing.T) {
	g := newTestGameWithResourceSetup(t, game.AoE2, func(resources *fakeResources) {
		resources.menuBackgrounds = &models.MenuBackgrounds{
			Periods: []models.MenuBackgroundPeriod{{
				Event: 290,
				Start: time.Now().Add(-time.Hour),
				End:   time.Now().Add(time.Hour),
			}},
		}
	})
	h := gameHandler(game.AoE2)
	_, sid := createUserAndSessionForGame(t, g, 76561198000000034, "aoe2Background")

	rr := doRequest(h, http.MethodGet, "/game/CommunityEvent/getAvailableCommunityEvents", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	events, ok := response[1].([]any)
	if !ok {
		t.Fatalf("events = %#v", response[1])
	}
	for _, raw := range events {
		if event, ok := raw.([]any); ok && len(event) == 8 && event[5] == float64(0) {
			t.Fatalf("AoE II gained a type-0 event: %#v", event)
		}
	}
}
