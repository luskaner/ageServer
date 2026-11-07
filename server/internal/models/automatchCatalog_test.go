package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
)

// bundledResponses loads the real resources of a title into the in-memory
// Resources fake used by the package tests.
func bundledResources(t *testing.T, gameID string, files ...string) *fakeResources {
	t.Helper()
	resources := &fakeResources{}
	resources.Initialize(gameID, nil)
	base := filepath.Join("..", "..", "resources", "responses", gameID)
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatalf("read %s/%s: %v", gameID, name, err)
		}
		var decoded i.A
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode %s/%s: %v", gameID, name, err)
		}
		resources.arrayFiles[name] = decoded
	}
	return resources
}

func bundledLeaderboards(t *testing.T, gameID string) *MainLeaderboardDefinitions {
	t.Helper()
	resources := bundledResources(t, gameID, "leaderboards.json")
	definitions := &MainLeaderboardDefinitions{}
	definitions.Initialize(gameID, resources.arrayFiles["leaderboards.json"])
	return definitions
}

func TestLoadQueueCatalogAge2FromBundledPool(t *testing.T) {
	resources := bundledResources(t, "age2", "automatchMaps.json", "automatchQueues.json")
	catalog, err := LoadQueueCatalog("age2", resources, bundledLeaderboards(t, "age2"))
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	oneVsOne, ok := catalog.Queue(6)
	if !ok {
		t.Fatal("pool 6 missing")
	}
	if oneVsOne.Teams != 2 || oneVsOne.MaxPlayers != 2 || oneVsOne.TeamSize != 1 {
		t.Fatalf("pool 6 shape = teams %d players %d teamSize %d", oneVsOne.Teams, oneVsOne.MaxPlayers, oneVsOne.TeamSize)
	}
	if !oneVsOne.Rated || oneVsOne.LeaderboardID != SoloRMRanked {
		t.Fatalf("pool 6 rating = rated %t ladder %d", oneVsOne.Rated, oneVsOne.LeaderboardID)
	}
	if oneVsOne.MatchCompletionValue != 225 || oneVsOne.UpdateStatusAck != 0 {
		t.Fatalf("pool 6 completion/ack = %d/%d", oneVsOne.MatchCompletionValue, oneVsOne.UpdateStatusAck)
	}
	if len(oneVsOne.Maps) != 7 {
		t.Fatalf("pool 6 maps = %d, want 7", len(oneVsOne.Maps))
	}
	team, ok := catalog.Queue(7)
	if !ok || team.MaxPlayers != 4 || team.Teams != 2 {
		t.Fatalf("pool 7 = %+v", team)
	}
	coop, ok := catalog.Queue(57)
	if !ok || coop.Rated || coop.LeaderboardID != 0 {
		t.Fatalf("pool 57 should be unrated: %+v", coop)
	}
	// The old hardcoded list included 20 (a 3v3 family) and 1 (which does not
	// exist in the pool). Neither may resolve as 1v1 Random Map.
	if queue, ok := catalog.Queue(20); !ok || queue.MaxPlayers != 6 {
		t.Fatalf("pool 20 = %+v", queue)
	}
	if _, ok := catalog.Queue(1); ok {
		t.Fatal("matchType 1 must not exist in the age2 pool")
	}
	if queue, ok := catalog.Compatible([]int32{57}, []int32{58}); ok {
		t.Fatalf("different co-op families paired as %+v", queue)
	}
	if queue, ok := catalog.Compatible([]int32{6}, []int32{7, 6}); !ok || queue.ID != 6 {
		t.Fatalf("compatible 6/7 = %+v, %t", queue, ok)
	}
}

func TestLoadQueueCatalogAge4Overlay(t *testing.T) {
	resources := bundledResources(t, "age4", "automatchMaps.json", "automatchQueues.json")
	catalog, err := LoadQueueCatalog("age4", resources, bundledLeaderboards(t, "age4"))
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	ranked, ok := catalog.Queue(1)
	if !ok || !ranked.Rated || ranked.LeaderboardID != 1 {
		t.Fatalf("pool 1 = %+v", ranked)
	}
	if ranked.MatchCompletionValue != 1 || ranked.UpdateStatusAck != 30 {
		t.Fatalf("pool 1 completion/ack = %d/%d", ranked.MatchCompletionValue, ranked.UpdateStatusAck)
	}
	coop, ok := catalog.Queue(860)
	if !ok {
		t.Fatal("pool 860 missing")
	}
	if !coop.MatchTypeIsAdvertisement || coop.AdvertisementMaxPlayers() != 16 {
		t.Fatalf("pool 860 advertisement = %t/%d", coop.MatchTypeIsAdvertisement, coop.AdvertisementMaxPlayers())
	}
	if coop.AdvertisementMatchType(AutomatchAdvertisementMatchType) != 860 {
		t.Fatalf("pool 860 matchtype = %d", coop.AdvertisementMatchType(AutomatchAdvertisementMatchType))
	}
}

func TestLoadQueueCatalogRejectsUnknownOverlayMatchType(t *testing.T) {
	resources := &fakeResources{}
	resources.Initialize("", nil)
	resources.arrayFiles["automatchMaps.json"] = i.A{0, i.A{
		i.A{1, "Arabia.rms", 2, 2, 6, nil, nil, ""},
	}}
	resources.arrayFiles["automatchQueues.json"] = i.A{0, i.A{
		map[string]any{"matchTypes": []any{float64(999)}, "ladder": float64(3), "rated": true},
	}}
	if _, err := LoadQueueCatalog("age2", resources, nil); err == nil {
		t.Fatal("expected an error for an overlay matchType missing from the pool")
	}
}

func TestLoadQueueCatalogRejectsHeterogeneousPool(t *testing.T) {
	resources := &fakeResources{}
	resources.Initialize("", nil)
	resources.arrayFiles["automatchMaps.json"] = i.A{0, i.A{
		i.A{1, "Arabia.rms", 2, 2, 6, nil, nil, ""},
		i.A{2, "Arena.rms", 2, 4, 6, nil, nil, ""},
	}}
	if _, err := LoadQueueCatalog("age2", resources, nil); err == nil {
		t.Fatal("expected an error for a non-homogeneous pool")
	}
}

func TestLoadQueueCatalogWithoutPoolHasNoQueues(t *testing.T) {
	resources := &fakeResources{}
	resources.Initialize("", nil)
	catalog, err := LoadQueueCatalog("age1", resources, nil)
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	if catalog.Len() != 0 {
		t.Fatalf("queues = %d, want 0", catalog.Len())
	}
}

func TestAssumeQueueFallbackIsUnrated(t *testing.T) {
	queue := assumeQueueFallback("age2")
	if queue.Rated || queue.LeaderboardID != 0 {
		t.Fatalf("fallback must be unrated, got %+v", queue)
	}
	if queue.MaxPlayers != 2 || queue.Teams != 2 {
		t.Fatalf("fallback shape = %+v", queue)
	}
}

func TestEncodeUnmatchedStatusesEchoesRaceIDs(t *testing.T) {
	statuses := encodeUnmatchedAutomatchQueueStatuses([]int32{57, 58, 59}, []int32{0, 1, 2}, 1)
	if len(statuses) != 3 {
		t.Fatalf("statuses = %d", len(statuses))
	}
	for index, row := range statuses {
		if row[0] != strconv.Itoa(57+index) {
			t.Fatalf("status %d matchType = %#v", index, row[0])
		}
		if row[1] != 1 {
			t.Fatalf("status %d state = %#v, want 1", index, row[1])
		}
		mapRow, ok := row[2].(i.A)
		if !ok || len(mapRow) != 1 {
			t.Fatalf("status %d map = %#v", index, row[2])
		}
		pair, ok := mapRow[0].(i.A)
		if !ok || pair[0] != index || pair[1] != 1 {
			t.Fatalf("status %d pair = %#v", index, mapRow[0])
		}
	}
}

func strconvID(id int32) string {
	return strconv.Itoa(int(id))
}

func TestAutomatchUnmatchedResponseGolden(t *testing.T) {
	response := automatchUnmatchedResponse(1001, []int32{57, 58, 59}, []int32{0, 0, 0}, 1, 0)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `[0,[["57",1,[[0,1]],"0"],["58",1,[[0,1]],"0"],["59",1,[[0,1]],"0"]],1001,null,-1,"",-1,-1,-1,"0","0","-1",-1,"authtoken","0.0.0.0",0,0,0,"",[],null,0]`
	if string(encoded) != want {
		t.Fatalf("unmatched response:\n got %s\nwant %s", encoded, want)
	}
}

// The client renders polling response slot 2 as an H:MM:SS estimated wait:
// a random token made Quick Play show "157:45:32" (567932821 ms). The LAN
// server cannot predict waits, so the estimate must stay 0 (00:00:00).
func TestEstimatedWaitMillisIsZero(t *testing.T) {
	if got := estimatedWaitMillis(); got != 0 {
		t.Fatalf("estimated wait = %d ms, want 0 (rendered by the client as H:MM:SS)", got)
	}
}

func TestAutomatchUnmatchedResponseCarriesAssignedMatchID(t *testing.T) {
	response := automatchUnmatchedResponse(1001, []int32{57}, []int32{0}, 0, 508712036)
	if response[12] != int32(508712036) {
		t.Fatalf("slot 12 = %#v, want the assigned matchID", response[11])
	}
}

func TestEncodeMatchedResponseGoldenShape(t *testing.T) {
	peers := []i.A{
		{508712036, 19375011, -1, 4220944, 0, 0, "/10.0.11.16"},
	}
	response := encodeMatchedResponse(automatchMatchedParams{
		EstimatedWaitMs: 1001,
		MatchTypes:      []int32{6},
		RaceIDs:         []int32{0},
		HostProfileID:   19375011,
		MapName:         "Arabia.rms",
		MapID:           1,
		Teams:           2,
		Players:         2,
		QueueID:         6,
		MatchID:         508712036,
		IP:              "20.90.13.208",
		Ports:           []int{27014, 27114, 27214},
		Region:          "westeurope",
		Peers:           peers,
		CompletionValue: 225,
	})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `[0,[["6",0,[[0,1]],"0"]],1001,null,19375011,"Arabia.rms",0,0,1,"2","2","6",508712036,"authtoken","20.90.13.208",27014,27114,27214,"westeurope",[[508712036,19375011,-1,4220944,0,0,"/10.0.11.16"]],"",225]`
	if string(encoded) != want {
		t.Fatalf("matched response:\n got %s\nwant %s", encoded, want)
	}
}

func TestEncodeAutomatchGameMessageIsPositional(t *testing.T) {
	match := &automatchMatch{
		ID:      508712036,
		HostID:  5278568,
		MapName: "Fortress.rms",
		Queue:   &automatchQueue{Teams: 2, MaxPlayers: 2},
	}
	recipient := &automatchQueueEntry{RaceInfoRaces: []int32{0}}
	peers := []i.A{{508712036, 5278568, -1, 4220944, 0, 0, "/10.0.11.16"}}
	message := encodeAutomatchGameMessageLocked(match, recipient, "20.90.13.208", []int{27014, 27114, 27214}, peers)
	if len(message) != 15 {
		t.Fatalf("wss payload has %d fields, want 15", len(message))
	}
	if message[0] != int32(5278568) || message[1] != int32(508712036) || message[2] != "Fortress.rms" {
		t.Fatalf("wss head = %#v", message[:3])
	}
	if message[6] != "2" || message[7] != "2" {
		t.Fatalf("wss (teams, players) = %#v/%#v, want 2/2", message[6], message[7])
	}
	if message[14] != "" {
		t.Fatalf("wss tail = %#v, want empty string", message[14])
	}
}

func TestAssignTeamsKeepsPartiesTogether(t *testing.T) {
	queue := &automatchQueue{Teams: 2, MaxPlayers: 4, TeamSize: 2}
	partyA := &automatchPartyGroup{PartySessionID: 1, Entries: []*automatchQueueEntry{{UserID: 1}, {UserID: 2}}}
	partyB := &automatchPartyGroup{PartySessionID: 2, Entries: []*automatchQueueEntry{{UserID: 3}, {UserID: 4}}}
	players, teams, ok := assignTeams([]*automatchPartyGroup{partyA, partyB}, queue)
	if !ok || len(players) != 4 {
		t.Fatalf("assignTeams = %v %v %t", players, teams, ok)
	}
	if teams[0] != 0 || teams[1] != 0 || teams[2] != 1 || teams[3] != 1 {
		t.Fatalf("teams = %v", teams)
	}
	tooBig := &automatchPartyGroup{PartySessionID: 3, Entries: []*automatchQueueEntry{{UserID: 5}, {UserID: 6}, {UserID: 7}}}
	solo := &automatchPartyGroup{PartySessionID: 0, Entries: []*automatchQueueEntry{{UserID: 8}}}
	if _, _, ok := assignTeams([]*automatchPartyGroup{tooBig, solo}, queue); ok {
		t.Fatal("a three-player party must not be split into a two-player team")
	}
}

func TestIsAutomatchHostRequestPerTitle(t *testing.T) {
	cases := []struct {
		name        string
		gameTitle   string
		description string
		matchType   uint16
		pollID      int32
		want        bool
	}{
		// AoE4 Quick Match creates its party/session lobby with this exact
		// combination; treating it as a matchmaking host aborted the search.
		{"aoe4 party lobby", "age4", AutomatchMatchDescription, 19, -1, false},
		{"aoe4 match advertisement", "age4", AutomatchAutoMatchDescription, 860, -1, true},
		{"aoe4 with poll id", "age4", AutomatchMatchDescription, 19, 42, true},
		{"aoe2 matchmaking host", "age2", AutomatchMatchDescription, 19, -1, true},
		{"aoe2 party lobby", "age2", AutomatchMatchDescription, 1, -1, false},
		{"aoe2 with poll id", "age2", "", 1, 7, true},
		// Titles without a captured queue endpoint never claim a host request.
		{"age1", "age1", AutomatchMatchDescription, 19, -1, false},
		{"age3", "age3", AutomatchMatchDescription, 19, -1, false},
		{"aom", "athens", AutomatchMatchDescription, 19, -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAutomatchHostRequest(tc.gameTitle, tc.description, tc.matchType, tc.pollID); got != tc.want {
				t.Fatalf("IsAutomatchHostRequest(%q, %q, %d, %d) = %t, want %t",
					tc.gameTitle, tc.description, tc.matchType, tc.pollID, got, tc.want)
			}
		})
	}
}

func TestCancelMatchPreservesFIFOQueuedAt(t *testing.T) {
	host := &automatchQueueEntry{UserID: 1, QueuedAt: time.Unix(100, 0), MatchTypes: []int32{6}, Session: &SessionData{}}
	joiner := &automatchQueueEntry{UserID: 2, QueuedAt: time.Unix(200, 0), MatchTypes: []int32{6}, Session: &SessionData{}}
	match := &automatchMatch{ID: 7, Players: []*automatchQueueEntry{host, joiner}}
	mm := &MainMatchmaking{
		queue:   map[int32]*automatchQueueEntry{},
		byUser:  map[int32]*automatchQueueEntry{1: host, 2: joiner},
		byPoll:  map[int32]*automatchQueueEntry{},
		matches: map[int32]*automatchMatch{7: match},
	}
	mm.cancelMatchLocked(match, true, 0)
	if mm.queue[1].QueuedAt.Unix() != 100 || mm.queue[2].QueuedAt.Unix() != 200 {
		t.Fatalf("requeue reset FIFO: %v / %v", mm.queue[1].QueuedAt, mm.queue[2].QueuedAt)
	}
	if _, ok := mm.matches[7]; ok {
		t.Fatal("cancelled match still registered")
	}
}
