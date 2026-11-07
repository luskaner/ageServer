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

// bundledAoE1Resources loads the AoE1 automatch pool and overlay into the
// in-memory resources before the game is created, which is when the catalog is
// read.
func bundledAoE1Resources(t *testing.T) func(*fakeResources) {
	t.Helper()
	return func(fake *fakeResources) {
		for _, name := range []string{"automatchMaps.json", "automatchQueues.json"} {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "responses", "age1", name))
			if err != nil {
				t.Fatalf("read age1/%s: %v", name, err)
			}
			var decoded i.A
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("decode age1/%s: %v", name, err)
			}
			fake.arrayFiles[name] = decoded
		}
	}
}

// The AoE1 capture (F:\aoeApi\logs\aoe1, 2026-11-07) shows a 20-field polling
// body with the queue population in the second status field and the matchID in
// slot 12. Two players asking for the same pool must pair.
func TestAoE1QuickMatchPollingAndPairing(t *testing.T) {
	g := newTestGameWithResourceSetup(t, game.AoE1, bundledAoE1Resources(t))
	h := gameHandler(game.AoE1)
	host, hostSid := createUserAndSessionForGame(t, g, 76561198000000041, "aoe1Host")
	joiner, joinerSid := createUserAndSessionForGame(t, g, 76561198000000042, "aoe1Joiner")

	pollBody := func(userID int32, partySessionID int64) string {
		return fmt.Sprintf(
			"callNum=1&connect_id=x&matchTypes=[5]&partySessionID=%d&raceInfoRaceID=[17]"+
				"&raceInfoKey=[0]&raceInfoProfileID=[%d]&relayRegion=Server&versionFlags=0",
			partySessionID,
			userID,
		)
	}
	poll := func(sid string, userID int32, partySessionID int64) i.A {
		t.Helper()
		rr := doRequest(h, http.MethodPost, "/game/automatch2/polling", g, sid, pollBody(userID, partySessionID), nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("poll code = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
		}
		var response i.A
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("poll response is not JSON: %v body=%q", err, rr.Body.String())
		}
		return response
	}

	// AoE1 has no party queue: each player sends its own session advertisement
	// id as partySessionID and the engine must treat every entry as a unit of
	// one (a shared id must not join them into an incomplete party either).
	const hostParty, joinerParty int64 = 5345071, 5345072

	// First player: alone in the queue, so the population is 1.
	first := poll(hostSid, host.GetId(), hostParty)
	if len(first) != 20 {
		t.Fatalf("aoe1 polling body has %d fields, want 20: %#v", len(first), first)
	}
	if assigned, _ := first[12].(float64); assigned != -1 {
		t.Fatalf("first poll slot 12 = %v, want -1", first[12])
	}
	status := first[1].([]any)[0].([]any)
	if status[0] != "5" || status[1] != float64(1) {
		t.Fatalf("first poll status = %#v, want [\"5\",1,...]", status)
	}

	// Second player joins the same pool: both leave the queue, so the
	// population drops to 0 and the joiner receives the reserved matchID.
	second := poll(joinerSid, joiner.GetId(), joinerParty)
	if len(second) != 20 {
		t.Fatalf("aoe1 polling body has %d fields, want 20: %#v", len(second), second)
	}
	secondStatus := second[1].([]any)[0].([]any)
	if secondStatus[1] != float64(0) {
		t.Fatalf("joiner queue population = %v, want 0", secondStatus[1])
	}
	matchID, ok := second[12].(float64)
	if !ok || matchID <= 0 {
		t.Fatalf("joiner slot 12 = %#v, want the reserved matchID", second[12])
	}

	// The host finds the same matchID on its next poll and gets no host/map
	// data in the body (AoE1 carries it in the wss message).
	third := poll(hostSid, host.GetId(), hostParty)
	if third[12] != matchID {
		t.Fatalf("host slot 12 = %v, want %v", third[12], matchID)
	}
	if hostSlot, _ := third[4].(float64); hostSlot != -1 {
		t.Fatalf("aoe1 matched body leaked host: %#v", third[4])
	}
	if third[5] != "" {
		t.Fatalf("aoe1 matched body leaked map: %#v", third[5])
	}
}
