package userData

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestProfilesWithLanSuffix(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "Games", "Age of Empires 2 DE")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create active and server variants for same numeric ID
	if err := os.Mkdir(filepath.Join(profileDir, "12345678"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(profileDir, "87654321.lan"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(profileDir, "11111111.bak"), 0755); err != nil {
		t.Fatal(err)
	}
	// Non-numeric with suffix should still be skipped
	if err := os.Mkdir(filepath.Join(profileDir, "notanumber.lan"), 0755); err != nil {
		t.Fatal(err)
	}
	p := NewPath(dir, game.AoE2)
	err, profiles := p.Profiles()
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if profiles.Cardinality() != 3 {
		t.Errorf("expected 3 profiles (active + .lan + .bak), got %d: %v", profiles.Cardinality(), profiles.ToSlice())
	}
	// Verify types
	for _, d := range profiles.ToSlice() {
		switch d.Path() {
		case filepath.Join(profileDir, "12345678"):
			if d.Type() != TypeActive {
				t.Errorf("12345678 type %d want Active", d.Type())
			}
		case filepath.Join(profileDir, "87654321.lan"):
			if d.Type() != TypeServer {
				t.Errorf("87654321.lan type %d want Server", d.Type())
			}
		case filepath.Join(profileDir, "11111111.bak"):
			if d.Type() != TypeBackup {
				t.Errorf("11111111.bak type %d want Backup", d.Type())
			}
		}
	}
}

func TestSuffixDeterministic(t *testing.T) {
	if suffix(TypeServer) != ".lan" {
		t.Errorf("suffix Server = %q want .lan", suffix(TypeServer))
	}
	if suffix(TypeBackup) != ".bak" {
		t.Errorf("suffix Backup = %q want .bak", suffix(TypeBackup))
	}
	if suffix(TypeActive) != "" {
		t.Error("active suffix should be empty")
	}
	if suffix(999) != "" {
		t.Error("unknown type suffix should be empty")
	}
}

func TestTransformPathWithSuffixStripping(t *testing.T) {
	// active -> server
	ok, p := TransformPath("base/path.lan", TypeServer, TypeActive)
	if !ok || p != "base/path" {
		t.Errorf("server->active %v %q want base/path", ok, p)
	}
	ok, p = TransformPath("base/path", TypeActive, TypeBackup)
	if !ok || p != "base/path.bak" {
		t.Errorf("active->backup %v %q", ok, p)
	}
	ok, p = TransformPath("base/path.bak", TypeBackup, TypeServer)
	if !ok || p != "base/path.lan" {
		t.Errorf("backup->server %v %q", ok, p)
	}
	// mismatched src should fail
	if ok, _ := TransformPath("base/path", TypeServer, TypeBackup); ok {
		t.Error("should fail when src type mismatch")
	}
}
