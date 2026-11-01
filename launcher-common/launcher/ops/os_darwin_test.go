//go:build darwin

package ops

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/executor/custom"
	"github.com/luskaner/ageServer/common/game/executor/steam"
)

// Regression: the type assertion used steam.Exec (value) instead of
// *steam.Exec (pointer), so it never matched — MakeExec stores a *steam.Exec
// in the base.Executor interface, and Go type assertions distinguish between
// T and *T.
func TestNativeMacOsGameSteamAppleSilicon(t *testing.T) {
	c := &Config{gameId: game.AoE2}
	var executer base.Executor = &steam.Exec{}

	if !c.NativeMacOsGame(executer, false) {
		t.Fatal("AoE2 + Apple Silicon + native Steam must be recognized")
	}
}

func TestNativeMacOsGameCustomLauncher(t *testing.T) {
	c := &Config{gameId: game.AoE2}
	var executer base.Executor = custom.Exec{Executable: "/some/path"}

	if !c.NativeMacOsGame(executer, true) {
		t.Fatal("AoE2 + Apple Silicon + custom launcher must be recognized when considered")
	}
}

func TestNativeMacOsGameCustomLauncherNotConsidered(t *testing.T) {
	c := &Config{gameId: game.AoE2}
	var executer base.Executor = custom.Exec{Executable: "/some/path"}

	if c.NativeMacOsGame(executer, false) {
		t.Fatal("custom launcher must not match when considerCustomLauncher is false")
	}
}

func TestNativeMacOsGameOtherGames(t *testing.T) {
	c := &Config{gameId: game.AoE1}
	var executer base.Executor = &steam.Exec{}

	if c.NativeMacOsGame(executer, true) {
		t.Fatal("only AoE2 should match")
	}
}
