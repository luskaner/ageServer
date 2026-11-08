package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

func decodeArray(t *testing.T, rr interface{ Body() []byte }) i.A {
	t.Helper()
	var arr i.A
	if err := json.Unmarshal(rr.Body(), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return arr
}

func TestReportMatchExposesReplayDescriptors(t *testing.T) {
	cases := []struct {
		gameID       string
		wantSlots    int
		wantSlot3    bool
		wantResponse int
	}{
		{gameID: game.AoE2, wantSlots: 1, wantResponse: 16},
		{gameID: game.AoE4, wantSlots: 2, wantSlot3: true, wantResponse: 16},
		{gameID: game.AoE3, wantSlots: 0, wantResponse: 16},
	}
	for _, tc := range cases {
		t.Run(tc.gameID, func(t *testing.T) {
			g := newTestGame(t, tc.gameID)
			h := gameHandler(tc.gameID)
			sid := createSession(t, g)
			form := "match_id=555&profile_ids=[1714620840]&results=[1]&teamIDs=[0]&race_ids=[1]&xpGained=[0]"
			rr := doRequest(h, "POST", "/game/party/reportMatch", g, sid, form, nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("reportMatch got %d body=%q", rr.Code, rr.Body.String())
			}
			var response i.A
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response) != tc.wantResponse {
				t.Fatalf("response has %d fields, want %d", len(response), tc.wantResponse)
			}
			if response[0] != float64(0) {
				t.Fatalf("result code = %v, body %s", response[0], rr.Body.String())
			}
			for slot := 2; slot <= 3; slot++ {
				descriptor, ok := response[slot].([]any)
				if !ok || len(descriptor) != 6 {
					t.Fatalf("slot %d = %#v, want a descriptor", slot, response[slot])
				}
				wantDescriptor := tc.wantSlots == 2 || (tc.wantSlots == 1 && slot == 2)
				if !wantDescriptor {
					if descriptor[0] != float64(-1) {
						t.Fatalf("slot %d = %#v, want empty", slot, descriptor)
					}
					continue
				}
				if descriptor[0] != float64(555) {
					t.Fatalf("slot %d match id = %v", slot, descriptor[0])
				}
				if descriptor[4] != float64(0) {
					t.Fatalf("slot %d fifth element = %v, want 0 for multiplayer", slot, descriptor[4])
				}
				storageURL, _ := descriptor[1].(string)
				kind := slot - 2
				wantPath := fmt.Sprintf("/cloudfiles/replays/555/%d/", kind)
				if !strings.Contains(storageURL, wantPath) {
					t.Fatalf("slot %d url = %q, want %s", slot, storageURL, wantPath)
				}
				if strings.Contains(storageURL, "?") {
					t.Fatalf("slot %d url must not carry the SAS query: %q", slot, storageURL)
				}
				sig, _ := descriptor[5].(string)
				if !strings.Contains(sig, "sp=cw") || !strings.HasPrefix(sig, "sig=") {
					t.Fatalf("slot %d sig = %q", slot, sig)
				}
			}
		})
	}
}

func TestCreateOrReportSinglePlayerLifecycle(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	sid := createSession(t, g)

	rr := doRequest(h, "POST", "/game/party/createOrReportSinglePlayer", g, sid,
		"createMatchKey=1&matchTypeID=1&mapname=test-map&isComplete=0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("create got %d body=%q", rr.Code, rr.Body.String())
	}
	var created i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created) != 22 || created[0] != float64(0) {
		t.Fatalf("create response = %s", rr.Body.String())
	}
	matchID, _ := created[1].(float64)
	matchKey, _ := created[2].(string)
	if matchID == 0 || matchKey == "" {
		t.Fatalf("create returned matchID=%v matchKey=%q", created[1], created[2])
	}
	if created[12] != nil {
		t.Fatalf("create index 12 = %v, want null", created[12])
	}
	if slot, ok := created[13].([]any); !ok || slot[0] != float64(-1) {
		t.Fatalf("create must not issue upload credentials: %#v", created[13])
	}

	report := fmt.Sprintf(
		"createMatchKey=0&matchKey=%s&isComplete=1&results=[1]&teamIDs=[0]&race_ids=[1]&xpGained=[0]",
		matchKey,
	)
	rr = doRequest(h, "POST", "/game/party/createOrReportSinglePlayer", g, sid, report, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("report got %d body=%q", rr.Code, rr.Body.String())
	}
	var reported i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &reported); err != nil {
		t.Fatal(err)
	}
	if len(reported) != 22 || reported[0] != float64(0) {
		t.Fatalf("report response = %s", rr.Body.String())
	}
	if reported[1] != float64(0) || reported[2] != nil {
		t.Fatalf("report must not return a new match: %#v / %#v", reported[1], reported[2])
	}
	if reported[12] != "" {
		t.Fatalf("report index 12 = %v, want an empty string", reported[12])
	}
	for _, slot := range []int{13, 14} {
		descriptor, ok := reported[slot].([]any)
		if !ok || len(descriptor) != 6 || descriptor[4] != float64(1) {
			t.Fatalf("report slot %d = %#v, want a single-player descriptor", slot, reported[slot])
		}
		if id, _ := descriptor[0].(float64); id != matchID {
			t.Fatalf("report slot %d match id = %v, want %v", slot, descriptor[0], matchID)
		}
	}

	// A match reported as abandoned has no replay to upload.
	rr = doRequest(h, "POST", "/game/party/createOrReportSinglePlayer", g, sid,
		fmt.Sprintf("createMatchKey=0&matchKey=%s&isComplete=0", matchKey), nil)
	var abandoned i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &abandoned); err != nil {
		t.Fatal(err)
	}
	if abandoned[12] != nil {
		t.Fatalf("abandoned index 12 = %v, want null", abandoned[12])
	}
	for _, slot := range []int{13, 14} {
		if descriptor, ok := abandoned[slot].([]any); !ok || descriptor[0] != float64(-1) {
			t.Fatalf("abandoned slot %d = %#v, want empty", slot, abandoned[slot])
		}
	}
}

func TestCreateOrReportSinglePlayerUnknownKeyReturnsCode2(t *testing.T) {
	g := newTestGame(t, game.AoM)
	h := gameHandler(game.AoM)
	sid := createSession(t, g)
	rr := doRequest(h, "POST", "/game/party/createOrReportSinglePlayer", g, sid,
		"createMatchKey=0&matchKey=does-not-exist&isComplete=1", nil)
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 22 || response[0] != float64(2) {
		t.Fatalf("unknown matchKey response = %s", rr.Body.String())
	}
}

// TestGetRecentMatchHistoryListsEveryArtifact covers the age4 case: one entry
// per uploaded artifact, with the real kind in the fourth element. age2 and
// athens (multiplayer) only ever produce kind 0.
func TestGetRecentMatchHistoryListsEveryArtifact(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	sid := createSession(t, g)
	sess, ok := g.Sessions().GetById(sid)
	if !ok {
		t.Fatal("session not found")
	}
	profileID := sess.GetUserId()

	form := fmt.Sprintf(
		"match_id=555&profile_ids=[%d]&results=[1]&teamIDs=[0]&race_ids=[1]&xpGained=[0]",
		profileID,
	)
	if rr := doRequest(h, "POST", "/game/party/reportMatch", g, sid, form, nil); rr.Code != http.StatusOK {
		t.Fatalf("reportMatch got %d body=%q", rr.Code, rr.Body.String())
	}

	store := g.Resources().CloudFiles()
	for _, kind := range []int{0, 1} {
		key := fmt.Sprintf("replays/555/%d/aa.gz", kind)
		if _, err := store.Put(key, func(w io.Writer) error {
			_, err := w.Write([]byte("replay"))
			return err
		}); err != nil {
			t.Fatalf("Put kind %d: %v", kind, err)
		}
	}

	rr := doRequest(h, "GET",
		fmt.Sprintf("/game/Leaderboard/getRecentMatchHistory?profile_ids=[%d]", profileID),
		g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("getRecentMatchHistory got %d body=%q", rr.Code, rr.Body.String())
	}
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	matches, ok := response[1].([]any)
	if !ok {
		t.Fatalf("response[1] = %#v, want the match list", response[1])
	}
	var entry []any
	for _, candidate := range matches {
		fields, ok := candidate.([]any)
		if !ok || len(fields) < 14 {
			continue
		}
		if id, _ := fields[0].(float64); id == 555 {
			entry = fields
			break
		}
	}
	if entry == nil {
		t.Fatalf("match 555 missing from the history: %s", rr.Body.String())
	}
	urls, ok := entry[13].([]any)
	if !ok || len(urls) != 2 {
		t.Fatalf("matchUrls = %#v, want two entries", entry[13])
	}
	for index, wantKind := range []float64{0, 1} {
		artifact, ok := urls[index].([]any)
		if !ok || len(artifact) != 4 {
			t.Fatalf("matchUrls[%d] = %#v", index, urls[index])
		}
		if artifact[3] != wantKind {
			t.Fatalf("matchUrls[%d] kind = %v, want %v", index, artifact[3], wantKind)
		}
		url, _ := artifact[1].(string)
		wantPath := fmt.Sprintf("/cloudfiles/replays/555/%d/", int(wantKind))
		if !strings.Contains(url, wantPath) {
			t.Fatalf("matchUrls[%d] url = %q, want %s", index, url, wantPath)
		}
		if strings.Contains(url, "?") {
			t.Fatalf("matchUrls[%d] url must not carry the SAS query: %q", index, url)
		}
	}
}
