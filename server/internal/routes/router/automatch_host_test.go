package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// The AoE4 Quick Match screen creates its party/session lobby through
// /advertisement/host with the exact body captured in
// F:\ageLANServer\build\launcher\logs\age4\2026-11-06T23-51-33: description
// SESSION_MATCH_KEY, matchtype 19, map no_map, 8 players and no automatch poll
// id. That combination also matched the AoE2 matchmaking-host heuristic, so
// PrepareHost failed and the server answered error code 2; the client then
// logged "WorldwideAutomatch2Service::OnHostComplete - we're not polling at the
// moment, ignoring" and the search auto-cancelled.
func TestAoE4PartyLobbyHostIsNotMatchmaking(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	u, sid := createUserAndSessionForGame(t, g, 76561198000000021, "aoe4Searcher")

	body := fmt.Sprintf(
		"advertisementid=-1&appbinarychecksum=12260&automatchPoll_id=-1&datachecksum=-1717443686"+
			"&description=SESSION_MATCH_KEY&hostid=%d&isObservable=1&joinable=1&mapname=no_map"+
			"&matchtype=19&maxplayers=8&party=-1&race=9005920"+
			"&relayRegion=550e8400-e29b-41d4-a716-446655440000&serviceType=0&state=-1&team=-1&visible=0",
		u.GetId(),
	)
	rr := doRequest(h, http.MethodPost, "/game/advertisement/host", g, sid, body, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("host code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var arr i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatalf("host response is not JSON: %v body=%q", err, rr.Body.String())
	}
	if len(arr) == 0 {
		t.Fatalf("empty host response")
	}
	if code, ok := arr[0].(float64); !ok || int(code) != 0 {
		t.Fatalf("host result = %v, want 0 (the party lobby must be created) body=%q", arr[0], rr.Body.String())
	}
	if len(arr) < 2 {
		t.Fatalf("host response has no advertisement id: %q", rr.Body.String())
	}
	advID, ok := arr[1].(float64)
	if !ok || advID <= 0 {
		t.Fatalf("advertisement id = %v, want a positive id", arr[1])
	}
	// The lobby must be stored as a regular advertisement, not as a
	// server-managed matchmaking one: the client polls afterwards and the
	// engine resolves the party roster from this advertisement.
	adv, found := g.Advertisements().GetAdvertisement(int32(advID))
	if !found || adv == nil {
		t.Fatalf("advertisement %d was not stored", int32(advID))
	}
	if adv.UnsafeIsAutomatch() {
		t.Fatal("the AoE4 party lobby was stored as a matchmaking advertisement")
	}
	if adv.UnsafeGetMatchType() != 19 {
		t.Fatalf("matchtype = %d, want the client value 19", adv.UnsafeGetMatchType())
	}
	if adv.UnsafeGetMapName() != "no_map" {
		t.Fatalf("map = %q, want no_map", adv.UnsafeGetMapName())
	}
	if adv.UnsafeGetMaxPlayers() != 8 {
		t.Fatalf("maxplayers = %d, want 8", adv.UnsafeGetMaxPlayers())
	}
}

// The AoE4 search sequence is: create the party/session lobby, then poll until
// matched. Before the fix the lobby creation failed, so the client never
// polled and cancelled the search. This test walks the same sequence and
// checks that the search stays alive (unmatched, state 1) instead.
func TestAoE4SearchKeepsPollingAfterPartyLobby(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	u, sid := createUserAndSessionForGame(t, g, 76561198000000022, "aoe4Searcher2")

	hostBody := fmt.Sprintf(
		"advertisementid=-1&appbinarychecksum=12260&automatchPoll_id=-1&datachecksum=-1717443686"+
			"&description=SESSION_MATCH_KEY&hostid=%d&isObservable=1&joinable=1&mapname=no_map"+
			"&matchtype=19&maxplayers=8&party=-1&race=9005920"+
			"&relayRegion=550e8400-e29b-41d4-a716-446655440000&serviceType=0&state=-1&team=-1&visible=0",
		u.GetId(),
	)
	rr := doRequest(h, http.MethodPost, "/game/advertisement/host", g, sid, hostBody, nil)
	var hostResponse i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &hostResponse); err != nil || len(hostResponse) < 2 {
		t.Fatalf("host response = %q (%v)", rr.Body.String(), err)
	}
	if code, _ := hostResponse[0].(float64); int(code) != 0 {
		t.Fatalf("host result = %v, want 0", hostResponse[0])
	}
	partySessionID := int64(hostResponse[1].(float64))

	// The session travels in the query string (doRequest appends it); a
	// sessionID in the body would take precedence and fail the lookup.
	pollBody := fmt.Sprintf(
		"callNum=1&connect_id=x&matchTypes=[1]&partySessionID=%d&raceInfoRaceID=[106553]"+
			"&raceInfoKey=[0]&raceInfoProfileID=[%d]&relayRegion=Server&versionFlags=0",
		partySessionID,
		u.GetId(),
	)
	for attempt := 1; attempt <= 2; attempt++ {
		poll := doRequest(h, http.MethodPost, "/game/automatch2/polling", g, sid, pollBody, nil)
		var arr i.A
		if err := json.Unmarshal(poll.Body.Bytes(), &arr); err != nil {
			t.Fatalf("poll %d response is not JSON: %v body=%q", attempt, err, poll.Body.String())
		}
		if len(arr) != 22 {
			t.Fatalf("poll %d has %d fields, want 22: %q", attempt, len(arr), poll.Body.String())
		}
		if code, _ := arr[0].(float64); int(code) != 0 {
			t.Fatalf("poll %d result = %v, want 0", attempt, arr[0])
		}
		statuses, ok := arr[1].([]any)
		if !ok || len(statuses) != 1 {
			t.Fatalf("poll %d statuses = %#v", attempt, arr[1])
		}
		status, ok := statuses[0].([]any)
		if !ok || len(status) != 4 {
			t.Fatalf("poll %d status = %#v", attempt, statuses[0])
		}
		if status[0] != "1" {
			t.Fatalf("poll %d matchType echo = %#v, want \"1\"", attempt, status[0])
		}
		if state, _ := status[1].(float64); int(state) != 1 {
			t.Fatalf("poll %d state = %v, want 1 (still searching)", attempt, status[1])
		}
		// Slot 12 stays -1 while no match has been assigned.
		if assigned, _ := arr[12].(float64); int(assigned) != -1 {
			t.Fatalf("poll %d slot 12 = %v, want -1", attempt, arr[12])
		}
	}
}
