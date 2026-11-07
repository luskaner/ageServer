package models

import "testing"

func TestNormalizeMapName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Arabia.rms", "arabia"},
		{"Arabia", "arabia"},
		{"ARABIA.RMS2", "arabia"},
		{"Arabian Desert.rms", "arabian desert"},
		{"Arabian_Desert", "arabian desert"},
		{"  GoldenPit.rms2  ", "goldenpit"},
	}
	for _, test := range tests {
		if got := normalizeMapName(test.in); got != test.want {
			t.Errorf("normalizeMapName(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestSelectableMapsNoVetoes(t *testing.T) {
	candidates := testMapCandidates()
	allowed, fallback := selectableMaps(candidates, &automatchQueueEntry{}, &automatchQueueEntry{})
	if fallback {
		t.Fatal("unexpected fallback without vetoes")
	}
	if len(allowed) != len(candidates) {
		t.Fatalf("allowed = %d maps, want %d", len(allowed), len(candidates))
	}
}

func TestSelectableMapsExcludesBothPlayersVetoes(t *testing.T) {
	candidates := testMapCandidates()
	// AoE2 sends no vetoMapKey: the name applies to every queue.
	host := &automatchQueueEntry{VetoMapNames: []string{"Arabia"}}
	// AoE4 sends vetoMapKey as the index inside matchTypes, parallel to
	// vetoMaps. Key 0 is matchType 20 here, so "Arena" is only vetoed in 20.
	joiner := &automatchQueueEntry{
		MatchTypes:   []int32{20, 21},
		VetoMapKeys:  []int32{0},
		VetoMapNames: []string{"Arena"},
	}
	allowed, fallback := selectableMapsFor(20, candidates, []*automatchQueueEntry{host, joiner})
	if fallback {
		t.Fatal("unexpected fallback when maps remain")
	}
	for _, candidate := range allowed {
		if candidate.ID == 1 || candidate.ID == 3 {
			t.Fatalf("vetoed map %q (id %d) was selectable in queue 20", candidate.Name, candidate.ID)
		}
	}
	if len(allowed) != len(candidates)-2 {
		t.Fatalf("allowed = %d maps, want %d", len(allowed), len(candidates)-2)
	}
	// The same veto must not apply to the other queue the client asked for.
	other, fallback := selectableMapsFor(21, candidates, []*automatchQueueEntry{host, joiner})
	if fallback {
		t.Fatal("unexpected fallback in queue 21")
	}
	found := false
	for _, candidate := range other {
		if candidate.ID == 3 {
			found = true
		}
	}
	if !found {
		t.Fatal("Arena was vetoed in queue 21, but the client vetoed it for queue 20 only")
	}
}

func TestVetoesMapDoesNotTreatKeysAsMapIDs(t *testing.T) {
	// Regression: the AoE4 capture sends vetoMapKey=[0,0,0,1,1,1,...] with
	// four matchTypes. Reading those values as map IDs vetoed maps 0-3 in
	// every queue.
	entry := &automatchQueueEntry{
		MatchTypes:   []int32{20, 21, 22, 23},
		VetoMapKeys:  []int32{0, 0, 0, 1, 1, 1},
		VetoMapNames: []string{"scen_carmel_tiny", "scen_cliffsanity_tiny", "scen_cliffside_tiny", "scen_carmel_small", "scen_cliffsanity_small", "scen_cliffside_small"},
	}
	// Map IDs 1..3 exist in the AoE4 pool and must not be vetoed by key 0..3.
	for _, id := range []int32{1, 2, 3} {
		candidate := automatchMapCandidate{ID: id, Name: "scen_dry_arabia_tiny"}
		if entry.vetoesMap(20, candidate) {
			t.Fatalf("map id %d was vetoed by a vetoMapKey index", id)
		}
	}
	// The names still apply to their own queue.
	if !entry.vetoesMap(20, automatchMapCandidate{ID: 100, Name: "scen_carmel_tiny"}) {
		t.Fatal("the queue-20 veto was lost")
	}
	if entry.vetoesMap(22, automatchMapCandidate{ID: 100, Name: "scen_carmel_tiny"}) {
		t.Fatal("a queue-20 veto leaked into queue 22")
	}
	if !entry.vetoesMap(21, automatchMapCandidate{ID: 101, Name: "scen_carmel_small"}) {
		t.Fatal("the queue-21 veto was lost")
	}
}

func TestSelectableMapsFallsBackToHostAllowedMaps(t *testing.T) {
	candidates := testMapCandidates()
	host := &automatchQueueEntry{VetoMapNames: []string{"Arabia"}}
	joiner := &automatchQueueEntry{VetoMapNames: []string{"Arena", "Migration", "Arabian Desert", "FourLakes", "goldenpit", "hideout"}}
	allowed, fallback := selectableMaps(candidates, host, joiner)
	if !fallback {
		t.Fatal("expected fallback when every map is vetoed by one player")
	}
	if len(allowed) != len(candidates)-1 {
		t.Fatalf("allowed = %d maps, want %d", len(allowed), len(candidates)-1)
	}
	for _, candidate := range allowed {
		if candidate.Name == "Arabia.rms" {
			t.Fatal("host-vetoed map was returned in the host-allowed fallback")
		}
	}
}

func TestSelectableMapsFallsBackToAllMaps(t *testing.T) {
	candidates := testMapCandidates()
	host := &automatchQueueEntry{VetoMapNames: mapNames(candidates)}
	joiner := &automatchQueueEntry{VetoMapNames: mapNames(candidates)}
	allowed, fallback := selectableMaps(candidates, host, joiner)
	if !fallback {
		t.Fatal("expected fallback when every map is vetoed by both players")
	}
	if len(allowed) != len(candidates) {
		t.Fatalf("allowed = %d maps, want all %d", len(allowed), len(candidates))
	}
}

func testMapCandidates() []automatchMapCandidate {
	return []automatchMapCandidate{
		{ID: 1, Name: "Arabia.rms"},
		{ID: 3, Name: "Arena.rms"},
		{ID: 5, Name: "Migration.rms"},
		{ID: 7, Name: "Arabian Desert.rms"},
		{ID: 9, Name: "FourLakes.rms"},
		{ID: 11, Name: "goldenpit.rms2"},
		{ID: 13, Name: "hideout.rms2"},
	}
}

func mapNames(candidates []automatchMapCandidate) []string {
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.Name)
	}
	return names
}
