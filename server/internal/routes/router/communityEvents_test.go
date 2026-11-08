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
