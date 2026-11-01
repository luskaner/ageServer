package ops

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestGameRequiresBattleServer(t *testing.T) {
	tests := []struct {
		gameId string
		want   bool
	}{
		{game.AoM, true},
		{game.AoE4, true},
		{game.AoE1, false},
		{game.AoE2, false},
		{game.AoE3, false},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		c := &Config{gameId: tt.gameId}
		got := c.gameRequiresBattleServer()
		if got != tt.want {
			t.Errorf("gameRequiresBattleServer(%q) = %v, want %v", tt.gameId, got, tt.want)
		}
	}
}
