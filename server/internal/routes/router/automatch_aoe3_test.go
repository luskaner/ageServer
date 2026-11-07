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

// bundledAoE3Resources loads the AoE3 automatch pool and overlay into the
// in-memory resources before the game is created.
func bundledAoE3Resources(t *testing.T) func(*fakeResources) {
	t.Helper()
	return func(fake *fakeResources) {
		for _, name := range []string{"automatchMaps.json", "automatchQueues.json"} {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "age3", name))
			if err != nil {
				t.Fatalf("read age3/%s: %v", name, err)
			}
			var decoded i.A
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("decode age3/%s: %v", name, err)
			}
			fake.arrayFiles[name] = decoded
		}
	}
}

// The AoE3 party/session lobby is SESSION_MATCH_KEY with matchtype 13. It used
// to be rejected because the title was not in the supported list, which aborted
// the whole ranked search.
func TestAoE3PartyLobbyHostIsNotMatchmaking(t *testing.T) {
	g := newTestGame(t, game.AoE3)
	h := gameHandler(game.AoE3)
	u, sid := createUserAndSessionForGame(t, g, 76561198000000051, "aoe3Searcher")

	body := fmt.Sprintf(
		"advertisementid=-1&appbinarychecksum=1289375459&automatchPoll_id=-1&datachecksum=728963967"+
			"&description=SESSION_MATCH_KEY&hostid=%d&isObservable=1&joinable=1&mapname="+
			"&matchtype=13&maxplayers=4&party=-1&race=0"+
			"&relayRegion=550e8400-e29b-41d4-a716-446655440000&serviceType=0&state=-1&team=-1&visible=0",
		u.GetId(),
	)
	rr := doRequest(h, http.MethodPost, "/game/advertisement/host", g, sid, body, nil)
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("host response is not JSON: %v body=%q", err, rr.Body.String())
	}
	if code, _ := response[0].(float64); int(code) != 0 {
		t.Fatalf("host result = %v, want 0 (body=%q)", response[0], rr.Body.String())
	}
	adv, found := g.Advertisements().GetAdvertisement(int32(response[1].(float64)))
	if !found || adv == nil {
		t.Fatal("the AoE3 party lobby was not stored")
	}
	if adv.UnsafeIsAutomatch() {
		t.Fatal("the AoE3 party lobby was stored as a matchmaking advertisement")
	}
	if adv.UnsafeGetMatchType() != 13 {
		t.Fatalf("matchtype = %d, want the client value 13", adv.UnsafeGetMatchType())
	}
}

// Two players asking for the same ranked pool pair. The polling body has 22
// fields and only carries the reserved matchID (host/map travel in the wss
// message), matching the 2026-11-07 capture.
func TestAoE3QuickMatchPollingAndPairing(t *testing.T) {
	g := newTestGameWithResourceSetup(t, game.AoE3, bundledAoE3Resources(t))
	h := gameHandler(game.AoE3)
	host, hostSid := createUserAndSessionForGame(t, g, 76561198000000052, "aoe3Host")
	joiner, joinerSid := createUserAndSessionForGame(t, g, 76561198000000053, "aoe3Joiner")

	poll := func(sid string, userID int32) i.A {
		body := fmt.Sprintf(
			"callNum=1&connect_id=x&matchTypes=[1]&partySessionID=0&raceInfoRaceID=[0]"+
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
		t.Fatalf("aoe3 polling body has %d fields, want 22: %#v", len(first), first)
	}
	status := first[1].([]any)[0].([]any)
	if status[0] != "1" || status[1] != float64(1) {
		t.Fatalf("first poll status = %#v, want [\"1\",1,...]", status)
	}

	second := poll(joinerSid, joiner.GetId())
	if secondStatus := second[1].([]any)[0].([]any); secondStatus[1] != float64(0) {
		t.Fatalf("joiner queue population = %v, want 0", secondStatus[1])
	}
	matchID, ok := second[12].(float64)
	if !ok || matchID <= 0 {
		t.Fatalf("joiner slot 12 = %#v, want the reserved matchID", second[12])
	}

	third := poll(hostSid, host.GetId())
	if third[12] != matchID {
		t.Fatalf("host slot 12 = %v, want %v", third[12], matchID)
	}
	if hostSlot, _ := third[4].(float64); hostSlot != -1 {
		t.Fatalf("aoe3 matched body leaked host: %#v", third[4])
	}
	if third[5] != "" {
		t.Fatalf("aoe3 matched body leaked map: %#v", third[5])
	}
}
