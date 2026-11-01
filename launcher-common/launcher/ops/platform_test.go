package ops

import (
	"errors"
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/executor/base"
)

func TestAdminErrorWithOtherError(t *testing.T) {
	result := &exec.Result{Err: errors.New("some other error")}
	if adminError(result) {
		t.Error("expected false for unrelated error")
	}
}

func TestAdminErrorWithNilError(t *testing.T) {
	result := &exec.Result{}
	if adminError(result) {
		t.Error("expected false for nil error")
	}
}

func TestNativeMacOsGameOnNonDarwin(t *testing.T) {
	c := &Config{}
	if c.NativeMacOsGame(nil, false) {
		t.Error("NativeMacOsGame should return false on non-darwin")
	}
	if c.NativeMacOsGame(nil, true) {
		t.Error("NativeMacOsGame should return false on non-darwin regardless of flag")
	}
}

func TestGamePathToGameCertPathOnNonDarwin(t *testing.T) {
	c := &Config{}
	var exec base.Executor
	got := c.GamePathToGameCertPath(exec, "/some/path/cert.pem")
	if got != "/some/path/cert.pem" {
		t.Errorf("GamePathToGameCertPath should return path unchanged, got %q", got)
	}
	got = c.GamePathToGameCertPath(exec, "")
	if got != "" {
		t.Errorf("GamePathToGameCertPath should return empty string, got %q", got)
	}
}

func TestBattleServerRequiredOnNonDarwin(t *testing.T) {
	c := &Config{gameId: "age2"}
	var exec base.Executor
	if c.BattleServerRequired(exec) {
		t.Error("BattleServerRequired should be false for AoE2")
	}
	c.gameId = "athens"
	if !c.BattleServerRequired(exec) {
		t.Error("BattleServerRequired should be true for AoM")
	}
	c.gameId = "age4"
	if !c.BattleServerRequired(exec) {
		t.Error("BattleServerRequired should be true for AoE4")
	}
}

func TestSetGameId(t *testing.T) {
	c := &Config{}
	c.SetGameId("age2")
	if c.gameId != "age2" {
		t.Errorf("SetGameId: gameId = %q, want %q", c.gameId, "age2")
	}
	c.SetGameId("")
	if c.gameId != "" {
		t.Errorf("SetGameId empty: gameId = %q, want empty", c.gameId)
	}
}
