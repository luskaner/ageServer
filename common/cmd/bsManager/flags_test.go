package bsManager

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestStartFlagSetDefaults(t *testing.T) {
	values, flags := StartFlagSet([]string{"a", "b"})
	for name, want := range map[string]string{
		"game":          "",
		"logRoot":       "",
		"gameConfig":    "",
		"hideWindow":    "false",
		"force":         "false",
		"noErrExisting": "false",
	} {
		f := flags.Lookup(name)
		if f == nil {
			t.Fatalf("flag %q not registered", name)
		}
		if f.DefValue != want {
			t.Errorf("%s default = %q, want %q", name, f.DefValue, want)
		}
	}
	if got := flags.Lookup("gameConfig").Usage; !contains(got, "a") || !contains(got, "b") {
		t.Errorf("gameConfig usage must list config paths: %q", got)
	}
	_ = values
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestStartFlagSetParseShorthands(t *testing.T) {
	values, flags := StartFlagSet(nil)
	if err := flags.Parse([]string{"--game", game.AoE2, "-w", "-f", "-r", "--logRoot", "C:\\logs"}); err != nil {
		t.Fatal(err)
	}
	if values.GameId != game.AoE2 {
		t.Errorf("game = %q", values.GameId)
	}
	if !values.HideWindow || !values.Force || !values.NoErrExisting {
		t.Errorf("bools = %v/%v/%v", values.HideWindow, values.Force, values.NoErrExisting)
	}
	if values.LogRoot != "C:\\logs" {
		t.Errorf("logRoot = %q", values.LogRoot)
	}
}

func TestRemoveFlagSetParse(t *testing.T) {
	values, flags := RemoveFlagSet()
	if err := flags.Parse([]string{
		"-r", "eu-west",
		"--games=" + game.AoE1,
		"--games=" + game.AoE3,
	}); err != nil {
		t.Fatal(err)
	}
	if values.Region != "eu-west" {
		t.Errorf("region = %q", values.Region)
	}
	if len(values.GameIds) != 2 {
		t.Errorf("games = %v", values.GameIds)
	}
}
