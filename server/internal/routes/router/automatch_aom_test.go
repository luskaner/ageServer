package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// bundledAoMResources loads the AoM automatch pool and overlay into the
// in-memory resources before the game is created.
func bundledAoMResources(t *testing.T) func(*fakeResources) {
	t.Helper()
	return func(fake *fakeResources) {
		for _, name := range []string{"automatchMaps.json", "automatchQueues.json"} {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "athens", name))
			if err != nil {
				t.Fatalf("read athens/%s: %v", name, err)
			}
			var decoded i.A
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("decode athens/%s: %v", name, err)
			}
			fake.arrayFiles[name] = decoded
		}
	}
}

// The AoM party/session lobby is SESSION_MATCH_KEY with matchtype 25 (ranked)
// or 27 (quickmatch). It must not be treated as a matchmaking host, and it must
// not be rejected for being a match-related description.
func TestAoMPartyLobbyHostIsNotMatchmaking(t *testing.T) {
	g := newTestGame(t, game.AoM)
	h := gameHandler(game.AoM)
	u, sid := createUserAndSessionForGame(t, g, 76561198000000061, "aomSearcher")

	for _, matchType := range []int{25, 27} {
		body := fmt.Sprintf(
			"advertisementid=-1&appbinarychecksum=-744760733&automatchPoll_id=-1&datachecksum=-1948075916"+
				"&description=SESSION_MATCH_KEY&hostid=%d&isObservable=1&joinable=1&mapname="+
				"&matchtype=%d&maxplayers=4&party=-1&race=0"+
				"&relayRegion=550e8400-e29b-41d4-a716-446655440000&serviceType=0&state=-1&team=-1&visible=0",
			u.GetId(),
			matchType,
		)
		rr := doRequest(h, http.MethodPost, "/game/advertisement/host", g, sid, body, nil)
		var response i.A
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("host response is not JSON: %v body=%q", err, rr.Body.String())
		}
		if code, _ := response[0].(float64); int(code) != 0 {
			t.Fatalf("matchtype %d host result = %v (body=%q)", matchType, response[0], rr.Body.String())
		}
		adv, found := g.Advertisements().GetAdvertisement(int32(response[1].(float64)))
		if !found || adv == nil {
			t.Fatalf("matchtype %d lobby was not stored", matchType)
		}
		if adv.UnsafeIsAutomatch() {
			t.Fatalf("matchtype %d lobby stored as matchmaking", matchType)
		}
	}
}

// Two players asking for the same ranked pool pair. AoM keeps the relay
// payload in the matched polling body (unlike AoE1/AoE3).
func TestAoMQuickMatchPollingAndPairing(t *testing.T) {
	g := newTestGameWithResourceSetup(t, game.AoM, bundledAoMResources(t))
	h := gameHandler(game.AoM)
	host, hostSid := createUserAndSessionForGame(t, g, 76561198000000062, "aomHost")
	joiner, joinerSid := createUserAndSessionForGame(t, g, 76561198000000063, "aomJoiner")

	poll := func(sid string, userID int32) i.A {
		body := fmt.Sprintf(
			"callNum=1&connect_id=x&matchTypes=[28]&partySessionID=0&raceInfoRaceID=[1]"+
				"&raceInfoKey=[0]&raceInfoProfileID=[%d]&relayRegion=Server&versionFlags=0",
			userID,
		)
		rr := doRequest(h, http.MethodPost, "/game/automatch2/polling", g, sid, body, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("poll code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
		}
		var response i.A
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("poll response is not JSON: %v body=%q", err, rr.Body.String())
		}
		return response
	}

	first := poll(hostSid, host.GetId())
	if len(first) != 22 {
		t.Fatalf("aom polling body has %d fields, want 22: %#v", len(first), first)
	}
	status := first[1].([]any)[0].([]any)
	if status[0] != "28" || status[1] != float64(1) {
		t.Fatalf("first poll status = %#v, want [\"28\",1,...]", status)
	}

	second := poll(joinerSid, joiner.GetId())
	if secondStatus := second[1].([]any)[0].([]any); secondStatus[1] != float64(0) {
		t.Fatalf("joiner queue population = %v, want 0", secondStatus[1])
	}
	matchID, ok := second[12].(float64)
	if !ok || matchID <= 0 {
		t.Fatalf("joiner slot 12 = %#v, want the reserved matchID", second[12])
	}
	// While the host has not created the advertisement the joiner only gets
	// the reserved id (the captured first AoM match behaves the same way).
	if hostSlot, _ := second[4].(float64); hostSlot != -1 {
		t.Fatalf("waiting-host joiner slot 4 = %#v, want -1", second[4])
	}

	// The host finds the same reserved id on its next poll. The relay payload
	// (host, rm_<map>, peers) is encoded by TestEncodeMatchedResponseGoldenShape
	// and only appears once the reserved advertisement exists.
	third := poll(hostSid, host.GetId())
	if third[12] != matchID {
		t.Fatalf("host slot 12 = %v, want %v", third[12], matchID)
	}
}
