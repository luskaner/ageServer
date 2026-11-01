package battleServer

import (
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		gameId string
		wantOk bool
		want   string
	}{
		{game.AoE1, true, Executable},
		{game.AoE2, true, filepath.Join("BattleServer", Executable)},
		{game.AoE3, true, Executable},
		{game.AoE4, false, ""},
		{game.AoM, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.gameId, func(t *testing.T) {
			ok, path := ResolvePath(tt.gameId)
			if ok != tt.wantOk {
				t.Errorf("ResolvePath(%q) ok = %v, want %v", tt.gameId, ok, tt.wantOk)
			}
			if path != tt.want {
				t.Errorf("ResolvePath(%q) path = %q, want %q", tt.gameId, path, tt.want)
			}
		})
	}
}

func TestResolvePathPriority(t *testing.T) {
	tests := []struct {
		gameId     string
		wantPriority []string
	}{
		{game.AoE1, []string{game.AoE1, game.AoE2, game.AoE3}},
		{game.AoE3, []string{game.AoE2, game.AoE3, game.AoE1}},
		{game.AoE2, []string{game.AoE2}},
		{game.AoE4, []string{game.AoE4}},
		{game.AoM,  []string{game.AoM}},
	}
	for _, tt := range tests {
		t.Run(tt.gameId, func(t *testing.T) {
			got := ResolvePathPriority(tt.gameId)
			if len(got) != len(tt.wantPriority) {
				t.Fatalf("ResolvePathPriority(%q) len = %d, want %d", tt.gameId, len(got), len(tt.wantPriority))
			}
			for i, want := range tt.wantPriority {
				if got[i] != want {
					t.Errorf("ResolvePathPriority(%q)[%d] = %q, want %q", tt.gameId, i, got[i], want)
				}
			}
		})
	}
}
