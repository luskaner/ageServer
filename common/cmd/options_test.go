package cmd

import (
	"slices"
	"strings"
	"testing"

	commonGame "github.com/luskaner/ageServer/common/game"
	"github.com/spf13/pflag"
)

func TestGameVarCommand(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	values := &GameIdValues{}
	GameVarCommand(fs, values.GameIdRef())
	f := fs.Lookup("game")
	if f == nil {
		t.Fatal("--game not registered")
	}
	if err := fs.Parse([]string{"--game", "age2"}); err != nil {
		t.Fatal(err)
	}
	if values.GameId != "age2" {
		t.Fatalf("game = %q", values.GameId)
	}
}

func TestGamesVarCommand(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	values := &[]string{}
	GamesVarCommand(fs, values)
	// stringArray semantics: the flag repeats, one value per occurrence.
	if err := fs.Parse([]string{"-e", commonGame.AoE1, "-e", commonGame.AoM}); err != nil {
		t.Fatal(err)
	}
	if len(*values) != 2 || !slices.Contains(*values, commonGame.AoE1) || !slices.Contains(*values, commonGame.AoM) {
		t.Fatalf("games = %v", *values)
	}
	def := fs.Lookup("games").DefValue
	for _, g := range []string{commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoE4, commonGame.AoM} {
		if !strings.Contains(def, g) {
			t.Errorf("default games missing %q: %q", g, def)
		}
	}
}

func TestLogRootCommand(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	values := &LogRootValues{}
	LogRootCommand(fs, values.LogRootRef())
	if fs.Lookup("logRoot") == nil {
		t.Fatal("--logRoot not registered")
	}
	if err := fs.Parse([]string{"--logRoot", "some/dir"}); err != nil {
		t.Fatal(err)
	}
	if values.LogRoot != "some/dir" {
		t.Fatalf("logRoot = %q", values.LogRoot)
	}
}

func TestGamesDescriptionListsAllSupportedGames(t *testing.T) {
	desc := gamesDescription()
	for _, g := range []string{commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoE4, commonGame.AoM} {
		if !strings.Contains(desc, g) {
			t.Errorf("description missing %q: %q", g, desc)
		}
	}
}

func TestCheckVersion(t *testing.T) {
	if !checkVersion(false, "1.2.3") {
		t.Fatal("without --version execution must continue (true)")
	}
	if checkVersion(true, "") || checkVersion(true, "1.2.3") {
		t.Fatal("with --version execution must stop (false)")
	}
}
