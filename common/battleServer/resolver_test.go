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
