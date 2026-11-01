package cert

import (
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
)

func TestHasCA(t *testing.T) {
	tests := []struct {
		gameId string
		want   bool
	}{
		{game.AoE1, false},
		{game.AoE2, true},
		{game.AoE3, true},
		{game.AoE4, false},
		{game.AoM, true},
	}
	for _, tt := range tests {
		t.Run(tt.gameId, func(t *testing.T) {
			if got := HasCA(tt.gameId); got != tt.want {
				t.Errorf("HasCA(%q) = %v, want %v", tt.gameId, got, tt.want)
			}
		})
	}
}

func TestNewCA_AoE2(t *testing.T) {
	ok, ca := NewCA(game.AoE2, "/games/aoe2")
	if !ok {
		t.Fatal("NewCA returned ok=false for AoE2")
	}
	expected := filepath.Join("/games/aoe2", "certificates")
	if ca.OriginalPath() != filepath.Join(expected, common.CACert) {
		t.Errorf("OriginalPath = %q, want %q", ca.OriginalPath(), filepath.Join(expected, common.CACert))
	}
	if ca.TmpPath() != filepath.Join(expected, common.CACert+".lan") {
		t.Errorf("TmpPath = %q, want %q", ca.TmpPath(), filepath.Join(expected, common.CACert+".lan"))
	}
	if ca.BackupPath() != filepath.Join(expected, common.CACert+".bak") {
		t.Errorf("BackupPath = %q, want %q", ca.BackupPath(), filepath.Join(expected, common.CACert+".bak"))
	}
}

func TestNewCA_AoE3(t *testing.T) {
	ok, ca := NewCA(game.AoE3, "/games/aoe3")
	if !ok {
		t.Fatal("NewCA returned ok=false for AoE3")
	}
	// AoE3 does NOT append "certificates"
	expected := "/games/aoe3"
	if ca.OriginalPath() != filepath.Join(expected, common.CACert) {
		t.Errorf("OriginalPath = %q, want %q", ca.OriginalPath(), filepath.Join(expected, common.CACert))
	}
}

func TestNewCA_NoCA(t *testing.T) {
	ok, _ := NewCA(game.AoE1, "/games/aoe1")
	if ok {
		t.Error("NewCA returned ok=true for AoE1 (no CA)")
	}
}

func TestCA_Name(t *testing.T) {
	_, ca := NewCA(game.AoE2, "/tmp")
	if ca.name() != common.CACert {
		t.Errorf("name() = %q, want %q", ca.name(), common.CACert)
	}
}
