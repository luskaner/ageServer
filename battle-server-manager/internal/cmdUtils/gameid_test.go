package cmdUtils

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

// Regression: ParsedGameIds used to return the exported game.SupportedGames
// singleton by reference; callers mutating the result (Pop) corrupted the
// global set for the rest of the process.
func TestParsedGameIdsDoesNotMutateGlobalSupportedGames(t *testing.T) {
	empty := []string{}
	before := game.SupportedGames.Cardinality()

	games, err := ParsedGameIds(&empty)
	if err != nil {
		t.Fatal(err)
	}
	if games.Cardinality() != before {
		t.Fatalf("returned set cardinality = %d, want %d", games.Cardinality(), before)
	}

	for i := 0; i < 3; i++ {
		games.Pop()
	}

	if after := game.SupportedGames.Cardinality(); after != before {
		t.Fatalf("global SupportedGames mutated: %d -> %d", before, after)
	}
}

func TestParsedGameIdsExplicitValid(t *testing.T) {
	ids := []string{game.AoE1, game.AoM}
	games, err := ParsedGameIds(&ids)
	if err != nil {
		t.Fatal(err)
	}
	if games.Cardinality() != 2 || !games.Contains(game.AoE1) || !games.Contains(game.AoM) {
		t.Fatalf("games = %v", games)
	}
}

func TestParsedGameIdsUnsupportedFails(t *testing.T) {
	ids := []string{game.AoE1, "not-a-game"}
	if _, err := ParsedGameIds(&ids); err == nil {
		t.Fatal("expected error for unsupported game")
	}
}

func TestParsedGameIdsNilUsesPackageVar(t *testing.T) {
	old := GameIds
	GameIds = []string{game.AoE2}
	defer func() { GameIds = old }()

	games, err := ParsedGameIds(nil)
	if err != nil {
		t.Fatal(err)
	}
	if games.Cardinality() != 1 || !games.Contains(game.AoE2) {
		t.Fatalf("games = %v", games)
	}
}
