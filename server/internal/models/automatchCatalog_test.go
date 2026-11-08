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
	statuses := encodeAutomatchQueueStatuses([]int32{57, 58, 59}, []int32{0, 1, 2}, []int{1, 2, 3})
	if len(statuses) != 3 {
		t.Fatalf("statuses = %d", len(statuses))
	}
	for index, row := range statuses {
		if row[0] != strconv.Itoa(57+index) {
			t.Fatalf("status %d matchType = %#v", index, row[0])
		}
		if row[1] != index+1 {
			t.Fatalf("status %d players = %#v, want %d", index, row[1], index+1)
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
	response := automatchUnmatchedResponse("", estimatedWaitMillis(), []int32{57, 58, 59}, []int32{0, 0, 0}, []int{1, 1, 1}, 0)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `[0,[["57",1,[[0,1]],"0"],["58",1,[[0,1]],"0"],["59",1,[[0,1]],"0"]],0,null,-1,"",-1,-1,-1,"0","0","-1",-1,"authtoken","0.0.0.0",0,0,0,"",[],null,0]`
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
	response := automatchUnmatchedResponse("", estimatedWaitMillis(), []int32{57}, []int32{0}, []int{0}, 508712036)
	if response[12] != int32(508712036) {
		t.Fatalf("slot 12 = %#v, want the assigned matchID", response[11])
	}
}

func TestEncodeMatchedResponseGoldenShape(t *testing.T) {
	peers := []i.A{
		{508712036, 19375011, -1, 4220944, 0, 0, "/10.0.11.16"},
	}
	response := encodeMatchedResponse(automatchMatchedParams{
		Title:           "age2",
		EstimatedWaitMs: 1001,
		MatchTypes:      []int32{6},
		RaceIDs:         []int32{0},
		Counts:          []int{0},
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
	message := encodeAutomatchGameMessageLocked("age2", match, recipient, "20.90.13.208", []int{27014, 27114, 27214}, peers)
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

func TestLoadQueueCatalogAge1FromBundledPool(t *testing.T) {
	resources := bundledResources(t, "age1", "automatchMaps.json", "automatchQueues.json")
	catalog, err := LoadQueueCatalog("age1", resources, nil)
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	if catalog.Len() != 8 {
		t.Fatalf("queues = %d, want the eight captured Quick Match queues", catalog.Len())
	}
	// matchTypes 1-4 are Random Map 1v1..4v4, 5-8 Death Match 1v1..4v4.
	for matchType, want := range map[int32]struct {
		players int
		ladder  int32
		mode    string
	}{
		1: {2, 1, "RANDOM"},
		2: {4, 1, "RANDOM"},
		3: {6, 1, "RANDOM"},
		4: {8, 1, "RANDOM"},
		5: {2, 3, "DEATHMATCH"},
		6: {4, 3, "DEATHMATCH"},
		7: {6, 3, "DEATHMATCH"},
		8: {8, 3, "DEATHMATCH"},
	} {
		queue, ok := catalog.Queue(matchType)
		if !ok {
			t.Fatalf("matchType %d missing from the age1 pool", matchType)
		}
		if queue.MaxPlayers != want.players || queue.Teams != 2 || queue.TeamSize != want.players/2 {
			t.Fatalf("matchType %d shape = %+v", matchType, queue)
		}
		if !queue.Rated || queue.LeaderboardID != want.ladder {
			t.Fatalf("matchType %d rating = rated %t ladder %d", matchType, queue.Rated, queue.LeaderboardID)
		}
		if !queue.MatchTypeIsAdvertisement {
			t.Fatalf("matchType %d must use the queue id as advertisement matchtype", matchType)
		}
		if len(queue.Maps) != 1 || queue.Maps[0].Name != want.mode {
			t.Fatalf("matchType %d maps = %+v, want %q", matchType, queue.Maps, want.mode)
		}
	}
	// "Any" is the client sending several matchTypes at once: they resolve to
	// the first queue of the catalog the player asked for.
	if queue, ok := catalog.QueueForMatchTypes([]int32{5, 6, 7, 8}); !ok || queue.ID != 5 {
		t.Fatalf("Any DM queue = %+v, %t; want the 1v1 Death Match pool", queue, ok)
	}
	if queue, ok := catalog.QueueForMatchTypes([]int32{2, 3, 4}); !ok || queue.ID != 2 {
		t.Fatalf("Any RM queue = %+v, %t; want the 2v2 Random Map pool", queue, ok)
	}
}

func TestAoE1UnmatchedResponseHasTwentyFields(t *testing.T) {
	response := automatchUnmatchedResponse("age1", 0, []int32{5, 6}, []int32{17, 17}, []int{1, 2}, 0)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `[0,[["5",1,[[17,1]],"0"],["6",2,[[17,1]],"0"]],0,null,-1,"",-1,-1,-1,"0","0","-1",-1,"authtoken","0.0.0.0",0,0,"",[],null]`
	if string(encoded) != want {
		t.Fatalf("aoe1 unmatched response:\n got %s\nwant %s", encoded, want)
	}
}

func TestAoE1MatchedResponseOnlyCarriesMatchID(t *testing.T) {
	response := encodeMatchedResponse(automatchMatchedParams{
		Title:      "age1",
		MatchTypes: []int32{5},
		RaceIDs:    []int32{17},
		Counts:     []int{0},
		MatchID:    5345077,
	})
	if len(response) != 20 {
		t.Fatalf("aoe1 matched response has %d fields, want 20", len(response))
	}
	if response[12] != int32(5345077) {
		t.Fatalf("slot 12 = %#v, want the matchID", response[12])
	}
	if response[4] != -1 || response[5] != "" {
		t.Fatalf("aoe1 matched response leaked host/map: %#v", response[4:6])
	}
}

func TestAoE1GameMessageShape(t *testing.T) {
	match := &automatchMatch{
		ID:      5345077,
		HostID:  4560039,
		MapName: "DEATHMATCH",
		Queue:   &automatchQueue{Teams: 2, MaxPlayers: 2, TeamSize: 1, Rated: true},
	}
	recipient := &automatchQueueEntry{RaceInfoRaces: []int32{17}}
	peers := []i.A{
		{5345077, 4560039, -1, 955103, 17, 0, "/10.0.11.16"},
		{5345077, 19375011, -1, 2154661, 17, 1, "/10.0.11.15"},
	}
	message := encodeAutomatchGameMessageLocked("age1", match, recipient, "51.124.194.157", []int{27012, 27112, 27212}, peers)
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	want := `[4560039,5345077,"DEATHMATCH",17,"1","0","2","1","authtoken","51.124.194.157",27012,27112,[[5345077,4560039,-1,955103,17,0,"/10.0.11.16"],[5345077,19375011,-1,2154661,17,1,"/10.0.11.15"]],""]`
	if string(encoded) != want {
		t.Fatalf("aoe1 wss payload:\n got %s\nwant %s", encoded, want)
	}
}

// Regression: sameParty only links positive party ids, so a solo entry used to
// produce an empty group and two solo players could never pair. The AoE1
// pairing test caught it through the HTTP route.
func TestSoloEntryFormsACompleteGroup(t *testing.T) {
	entry := &automatchQueueEntry{UserID: 1, MatchTypes: []int32{5}, QueuedAt: time.Unix(100, 0)}
	mm := &MainMatchmaking{queue: map[int32]*automatchQueueEntry{1: entry}}
	group := mm.groupForEntryLocked(entry, nil)
	if group == nil {
		t.Fatal("solo entry produced no group")
	}
	if len(group.Entries) != 1 || !group.Complete || group.RosterSize != 1 {
		t.Fatalf("solo group = %+v", group)
	}
	if !group.QueuedAt.Equal(entry.QueuedAt) {
		t.Fatalf("group queuedAt = %s, want %s", group.QueuedAt, entry.QueuedAt)
	}
	// The candidate filter must recognise the polling unit even when it is
	// wrapped by a different group object.
	if !groupContainsEntry(group, entry) {
		t.Fatal("groupContainsEntry did not find the polling entry")
	}
	if groupContainsEntry(&automatchPartyGroup{}, entry) {
		t.Fatal("groupContainsEntry matched an unrelated group")
	}
}

func TestUsesPartiesPerTitle(t *testing.T) {
	if usesPartiesForTitle("age1") {
		t.Fatal("AoE1 has no party queue: its entries must be units of one")
	}
	for _, title := range []string{"age2", "age4"} {
		if !usesPartiesForTitle(title) {
			t.Fatalf("%s queues parties as one unit", title)
		}
	}
}

func TestLoadQueueCatalogAge3FromBundledPool(t *testing.T) {
	resources := bundledResources(t, "age3", "automatchMaps.json", "automatchQueues.json")
	catalog, err := LoadQueueCatalog("age3", resources, bundledLeaderboards(t, "age3"))
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	if catalog.Len() != 12 {
		t.Fatalf("queues = %d, want the twelve ranked queues", catalog.Len())
	}
	// Supremacy is 1v1..4v4, Deathmatch 5..8 and Empire Wars 18..21, which is
	// the ladder catalogue of the title (matchType == ladder id).
	for matchType, want := range map[int32]struct {
		players int
		ladder  int32
	}{
		1:  {2, 1},
		2:  {4, 2},
		3:  {6, 3},
		4:  {8, 4},
		5:  {2, 5},
		6:  {4, 6},
		7:  {6, 7},
		8:  {8, 8},
		18: {2, 18},
		19: {4, 19},
		20: {6, 20},
		21: {8, 21},
	} {
		queue, ok := catalog.Queue(matchType)
		if !ok {
			t.Fatalf("matchType %d missing from the age3 pool", matchType)
		}
		if queue.MaxPlayers != want.players || queue.Teams != 2 || queue.TeamSize != want.players/2 {
			t.Fatalf("matchType %d shape = %+v", matchType, queue)
		}
		if !queue.Rated || queue.LeaderboardID != want.ladder {
			t.Fatalf("matchType %d rating = rated %t ladder %d", matchType, queue.Rated, queue.LeaderboardID)
		}
		if queue.Maps[0].Name != "RANDOM" {
			t.Fatalf("matchType %d map = %q, want RANDOM", matchType, queue.Maps[0].Name)
		}
	}
	// Several sizes at once (the screen lets the player pick more than one).
	if queue, ok := catalog.QueueForMatchTypes([]int32{3, 4}); !ok || queue.ID != 3 {
		t.Fatalf("3v3+4v4 = %+v, %t", queue, ok)
	}
}

func TestAoE3MatchedResponseHasNoRelayPayload(t *testing.T) {
	response := encodeMatchedResponse(automatchMatchedParams{
		Title:      "age3",
		MatchTypes: []int32{2},
		RaceIDs:    []int32{0},
		Counts:     []int{5},
		MatchID:    66655664,
	})
	// AoE3 goes through the early branch in matchedResponseLocked; the encoder
	// fallback for other titles keeps its 22-field shape.
	if len(response) != 22 {
		t.Fatalf("aoe3 matched response has %d fields, want 22", len(response))
	}
	if response[12] != int32(66655664) {
		t.Fatalf("slot 12 = %#v, want the matchID", response[12])
	}
	if response[4] != -1 || response[5] != "" {
		t.Fatalf("aoe3 matched response leaked host/map: %#v", response[4:6])
	}
}

func TestAoE3GameMessageShape(t *testing.T) {
	match := &automatchMatch{
		ID:      66655664,
		HostID:  18446739,
		MapName: "RANDOM",
		Queue:   &automatchQueue{Teams: 2, MaxPlayers: 4, TeamSize: 2, Rated: true},
	}
	recipient := &automatchQueueEntry{RaceInfoRaces: []int32{0}}
	peers := []i.A{
		{66655664, 18446739, 3632, 4093272, 0, 0, "/10.0.11.20"},
		{66655664, 19375011, -1, 5011221, 0, 0, "/10.0.11.16"},
		{66655664, 20215372, 4800, 6265019, 0, 1, "/10.0.11.15"},
		{66655664, 20216918, 4468, 6264875, 0, 1, "/10.0.11.15"},
	}
	message := encodeAutomatchGameMessageLocked("age3", match, recipient, "20.58.118.59", []int{27012, 27112, 27212}, peers)
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	// (teams, players per team): a 2v2 sends "2","2" with four players.
	want := `[18446739,66655664,"RANDOM",0,"0","0","2","2","authtoken","20.58.118.59",27012,27112,27212,[[66655664,18446739,3632,4093272,0,0,"/10.0.11.20"],[66655664,19375011,-1,5011221,0,0,"/10.0.11.16"],[66655664,20215372,4800,6265019,0,1,"/10.0.11.15"],[66655664,20216918,4468,6264875,0,1,"/10.0.11.15"]],""]`
	if string(encoded) != want {
		t.Fatalf("aoe3 wss payload:\n got %s\nwant %s", encoded, want)
	}
}

func TestLoadQueueCatalogAoMFromBundledPool(t *testing.T) {
	resources := bundledResources(t, "athens", "automatchMaps.json", "automatchQueues.json")
	catalog, err := LoadQueueCatalog("athens", resources, bundledLeaderboards(t, "athens"))
	if err != nil {
		t.Fatalf("LoadQueueCatalog: %v", err)
	}
	if catalog.Len() != 20 {
		t.Fatalf("queues = %d, want the eight ranked and twelve quickmatch queues", catalog.Len())
	}
	for matchType, players := range map[int32]int{
		1: 2, 2: 4, 3: 6, 4: 8, // Ranked Supremacy
		5: 2, 6: 4, 7: 6, 8: 8, // Ranked Deathmatch
		28: 2, 29: 4, 30: 6, 31: 8, // Quickmatch Supremacy
		32: 2, 33: 4, 34: 6, 35: 8, // Quickmatch Deathmatch
		36: 2, 37: 4, 38: 6, 39: 8, // Quickmatch Lightning
	} {
		queue, ok := catalog.Queue(matchType)
		if !ok {
			t.Fatalf("matchType %d missing from the athens pool", matchType)
		}
		if queue.MaxPlayers != players || queue.Teams != 2 || queue.TeamSize != players/2 {
			t.Fatalf("matchType %d shape = %+v", matchType, queue)
		}
		if !queue.Rated || queue.LeaderboardID != matchType {
			t.Fatalf("matchType %d ladder = %d rated %t", matchType, queue.LeaderboardID, queue.Rated)
		}
		if !queue.MatchTypeIsAdvertisement {
			t.Fatalf("matchType %d must use the queue id as advertisement matchtype", matchType)
		}
		// The captured advertisements name the scenario as rm_<map file>.
		if len(queue.Maps) == 0 || queue.Maps[0].Name[:3] != "rm_" {
			t.Fatalf("matchType %d maps = %+v", matchType, queue.Maps)
		}
	}
	// The client polls every quickmatch queue at once.
	if queue, ok := catalog.QueueForMatchTypes([]int32{28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39}); !ok || queue.ID != 28 {
		t.Fatalf("quickmatch any = %+v, %t", queue, ok)
	}
}

func TestAoMGameMessageShape(t *testing.T) {
	match := &automatchMatch{
		ID:      45351237,
		HostID:  1077703702,
		MapName: "rm_yellow_river",
		Queue:   &automatchQueue{Teams: 2, MaxPlayers: 2, TeamSize: 1, Rated: true},
		Players: []*automatchQueueEntry{{UserID: 1077703702}, {UserID: 1076940694}},
		TeamIDs: []int32{0, 1},
	}
	recipient := &automatchQueueEntry{UserID: 1076940694, RaceInfoRaces: []int32{1}}
	peers := []i.A{
		{45351237, 1077703702, -1, 3981859, 1, 0, "/10.0.11.12"},
		{45351237, 1076940694, -1, 3215409, 1, 1, "/10.0.11.14"},
	}
	message := encodeAutomatchGameMessageLocked("athens", match, recipient, "172.213.140.76", []int{27013, 27113, 27213}, peers)
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	// Slot 4 is the recipient team (1), slot 7 the players per team.
	want := `[1077703702,45351237,"rm_yellow_river",1,"1","0","2","1","authtoken","172.213.140.76",27013,27113,27213,[[45351237,1077703702,-1,3981859,1,0,"/10.0.11.12"],[45351237,1076940694,-1,3215409,1,1,"/10.0.11.14"]],""]`
	if string(encoded) != want {
		t.Fatalf("aom wss payload:\n got %s\nwant %s", encoded, want)
	}
}

// The profile table is the single source of truth for the differences the
// captures document. This test pins them so a refactor cannot silently change
// one title's wire shape.
func TestAutomatchTitleProfiles(t *testing.T) {
	cases := map[string]struct {
		pollingFields int
		gameFields    int
		relay         bool
		serverPeers   bool
		realTeams     bool
		teamSize      bool
		parties       bool
		reservation   string
	}{
		"age1":   {20, 14, false, true, true, true, false, AutomatchAutoMatchDescription},
		"age2":   {22, 15, true, false, false, false, true, AutomatchMatchDescription},
		"age3":   {22, 15, false, true, true, true, true, AutomatchAutoMatchDescription},
		"age4":   {22, 15, true, false, false, false, true, AutomatchAutoMatchDescription},
		"athens": {22, 15, true, true, true, true, true, AutomatchAutoMatchDescription},
	}
	for title, want := range cases {
		got := automatchProfile(title)
		if got.PollingFields != want.pollingFields || got.GameMessageFields != want.gameFields {
			t.Fatalf("%s wire shape = %d/%d, want %d/%d",
				title, got.PollingFields, got.GameMessageFields, want.pollingFields, want.gameFields)
		}
		if got.MatchedPayloadHasRelay != want.relay || got.ServerAddsPeers != want.serverPeers ||
			got.RealTeams != want.realTeams || got.GameMessageTeamSize != want.teamSize ||
			got.Parties != want.parties || got.ReservationDescription != want.reservation {
			t.Fatalf("%s profile = %+v, want %+v", title, got, want)
		}
	}
	// Every supported title must have a profile, and unknown titles fall back
	// to the shape the others started from.
	for _, title := range AutomatchGameTitles() {
		if _, ok := automatchProfiles[title]; !ok {
			t.Fatalf("%s is supported but has no profile", title)
		}
		if automatchProfile(title).PollingFields == 0 {
			t.Fatalf("%s has an empty profile", title)
		}
	}
	if fallback := automatchProfile("unknown"); fallback != automatchProfiles["age2"] {
		t.Fatalf("unknown title profile = %+v, want the AoE2 one", fallback)
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
		// AoE1 party/session lobby: SESSION_MATCH_KEY with matchtype 0.
		{"aoe1 party lobby", "age1", AutomatchMatchDescription, 0, -1, false},
		{"aoe1 match advertisement", "age1", AutomatchAutoMatchDescription, 5, -1, true},
		{"aoe1 with poll id", "age1", AutomatchMatchDescription, 0, 9, true},
		// AoE3 party/session lobby: SESSION_MATCH_KEY with matchtype 13.
		{"aoe3 party lobby", "age3", AutomatchMatchDescription, 13, -1, false},
		{"aoe3 match advertisement", "age3", AutomatchAutoMatchDescription, 2, -1, true},
		// AoM party/session lobby: SESSION_MATCH_KEY with matchtype 25 (ranked)
		// or 27 (quickmatch).
		{"aom party lobby", "athens", AutomatchMatchDescription, 27, -1, false},
		{"aom ranked lobby", "athens", AutomatchMatchDescription, 25, -1, false},
		{"aom match advertisement", "athens", AutomatchAutoMatchDescription, 28, -1, true},
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
