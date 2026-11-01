//go:build !windows

package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupportedGames(t *testing.T) {
	expected := []string{AoE1, AoE2, AoE3, AoE4, AoM}
	if SupportedGames.Cardinality() != len(expected) {
		t.Errorf("SupportedGames.Cardinality() = %d, want %d", SupportedGames.Cardinality(), len(expected))
	}
	for _, g := range expected {
		if !SupportedGames.Contains(g) {
			t.Errorf("SupportedGames does not contain %q", g)
		}
	}
}

func TestAllGamesEqualsSupportedGames(t *testing.T) {
	if AllGames.Cardinality() != SupportedGames.Cardinality() {
		t.Errorf("AllGames and SupportedGames have different sizes: %d vs %d",
			AllGames.Cardinality(), SupportedGames.Cardinality())
	}
}

func TestGameConstants(t *testing.T) {
	if AoE1 != "age1" {
		t.Errorf("AoE1 = %q, want %q", AoE1, "age1")
	}
	if AoE2 != "age2" {
		t.Errorf("AoE2 = %q, want %q", AoE2, "age2")
	}
	if AoE3 != "age3" {
		t.Errorf("AoE3 = %q, want %q", AoE3, "age3")
	}
	if AoE4 != "age4" {
		t.Errorf("AoE4 = %q, want %q", AoE4, "age4")
	}
	if AoM != "athens" {
		t.Errorf("AoM = %q, want %q", AoM, "athens")
	}
}

func TestFirstExistingFile_Found(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "exists.txt")
	os.WriteFile(f, []byte("hi"), 0644)

	got := FirstExistingFile([]string{filepath.Join(dir, "nope.txt"), f}, nil, func(fi os.FileInfo) bool {
		return !fi.IsDir()
	})
	if got != f {
		t.Errorf("FirstExistingFile = %q, want %q", got, f)
	}
}

func TestFirstExistingFile_NotFound(t *testing.T) {
	got := FirstExistingFile([]string{"/nonexistent/path"}, nil, func(fi os.FileInfo) bool {
		return !fi.IsDir()
	})
	if got != "" {
		t.Errorf("FirstExistingFile = %q, want empty", got)
	}
}

func TestFirstExistingFile_NilFileFn(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("data"), 0644)

	got := FirstExistingFile([]string{f}, nil, func(fi os.FileInfo) bool {
		return !fi.IsDir()
	})
	if got != f {
		t.Errorf("FirstExistingFile = %q, want %q", got, f)
	}
}

func TestFirstExistingFile_CustomFileFn(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("data"), 0644)

	got := FirstExistingFile([]string{"test.txt"}, func(s string) string {
		return filepath.Join(dir, s)
	}, func(fi os.FileInfo) bool {
		return !fi.IsDir()
	})
	if got != f {
		t.Errorf("FirstExistingFile = %q, want %q", got, f)
	}
}

func TestFirstExistingDir_Found(t *testing.T) {
	dir := t.TempDir()

	got := FirstExistingDir([]string{filepath.Join(dir, "nope"), dir}, nil)
	if got != dir {
		t.Errorf("FirstExistingDir = %q, want %q", got, dir)
	}
}

func TestFirstExistingDir_NotFound(t *testing.T) {
	got := FirstExistingDir([]string{"/nonexistent/dir"}, nil)
	if got != "" {
		t.Errorf("FirstExistingDir = %q, want empty", got)
	}
}

func TestFirstExistingFile_ExpansionError(t *testing.T) {
	got := FirstExistingFile([]string{"${UNCLOSED"}, nil, func(fi os.FileInfo) bool {
		return true
	})
	if got != "" {
		t.Errorf("FirstExistingFile with bad expansion = %q, want empty", got)
	}
}

func TestFirstExistingFile_SkipsDir(t *testing.T) {
	dir := t.TempDir()
	got := FirstExistingFile([]string{dir}, nil, func(fi os.FileInfo) bool {
		return !fi.IsDir()
	})
	if got != "" {
		t.Errorf("FirstExistingFile should skip directories, got %q", got)
	}
}
