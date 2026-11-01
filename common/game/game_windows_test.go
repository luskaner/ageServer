//go:build windows

package game

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSupportedGames(t *testing.T) {
	expected := []string{AoE1, AoE2, AoE3, AoE4, AoM}
	if SupportedGames.Cardinality() != len(expected) {
		t.Errorf("SupportedGames.Cardinality() = %d, want %d", SupportedGames.Cardinality(), len(expected))
	}
	for _, g := range expected {
		if !SupportedGames.Contains(g) {
			t.Errorf("SupportedGames does not contain %q", g)
		}
	}
}

func TestAllGamesEqualsSupportedGames(t *testing.T) {
	if AllGames.Cardinality() != SupportedGames.Cardinality() {
		t.Errorf("AllGames and SupportedGames have different sizes: %d vs %d",
			AllGames.Cardinality(), SupportedGames.Cardinality())
	}
}

func TestGameConstants(t *testing.T) {
	if AoE1 != "age1" {
		t.Errorf("AoE1 = %q, want %q", AoE1, "age1")
	}
	if AoE2 != "age2" {
		t.Errorf("AoE2 = %q, want %q", AoE2, "age2")
	}
	if AoE3 != "age3" {
		t.Errorf("AoE3 = %q, want %q", AoE3, "age3")
	}
	if AoE4 != "age4" {
		t.Errorf("AoE4 = %q, want %q", AoE4, "age4")
	}
	if AoM != "athens" {
		t.Errorf("AoM = %q, want %q", AoM, "athens")
	}
}

func TestUserProfilePathAoE4(t *testing.T) {
	path := UserProfilePath(AoE4)
	// KnownFolderPath(FOLDERID_Documents) should work on a normal Windows session
	if path == "" {
		t.Log("KnownFolderPath returned empty; may fail in service contexts")
	}
}

func TestUserProfilePathAoE4KnownFolderError(t *testing.T) {
	orig := knownFolderPathFn
	defer func() { knownFolderPathFn = orig }()
	knownFolderPathFn = func(*windows.KNOWNFOLDERID, uint32) (string, error) {
		return "", errors.New("known folder fail")
	}
	if got := UserProfilePath(AoE4); got != "" {
		t.Errorf("expected empty on KnownFolder error, got %q", got)
	}
}

func TestUserProfilePathOtherGames(t *testing.T) {
	want := os.Getenv("USERPROFILE")
	for _, g := range []string{AoE1, AoE2, AoE3, AoM} {
		if got := UserProfilePath(g); got != want {
			t.Errorf("UserProfilePath(%q) = %q, want USERPROFILE %q", g, got, want)
		}
	}
}
