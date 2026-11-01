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
	host := &automatchQueueEntry{VetoMapNames: []string{"Arabia"}}
	joiner := &automatchQueueEntry{VetoMapKeys: []int32{3}} // Arena
	allowed, fallback := selectableMaps(candidates, host, joiner)
	if fallback {
		t.Fatal("unexpected fallback when maps remain")
	}
	for _, candidate := range allowed {
		if candidate.ID == 1 || candidate.ID == 3 {
			t.Fatalf("vetoed map %q (id %d) was selectable", candidate.Name, candidate.ID)
		}
	}
	if len(allowed) != len(candidates)-2 {
		t.Fatalf("allowed = %d maps, want %d", len(allowed), len(candidates)-2)
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
