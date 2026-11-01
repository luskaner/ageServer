package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildResourcePath(t *testing.T) {
	got := BuildResourcePath("server")
	want := filepath.Join("build", "server", "resources")
	if got != want {
		t.Fatalf("BuildResourcePath(\"server\") = %q, want %q", got, want)
	}
}

func TestResourcePath(t *testing.T) {
	got := ResourcePath("launcher")
	want := filepath.Join("launcher", "resources")
	if got != want {
		t.Fatalf("ResourcePath(\"launcher\") = %q, want %q", got, want)
	}
}

func TestCopyGameConfigsRemovesStale(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)
	module := "testmod"
	srcDir := filepath.Join(tmp, module, "resources")
	os.MkdirAll(srcDir, 0755)
	srcFile := filepath.Join(srcDir, "config.game.toml")
	os.WriteFile(srcFile, []byte("game"), 0644)
	buildDir := filepath.Join(tmp, "build", module, "resources")
	os.MkdirAll(buildDir, 0755)
	staleFile := filepath.Join(buildDir, "config.deprecated.toml")
	os.WriteFile(staleFile, []byte("old"), 0644)
	mainFile := filepath.Join(buildDir, "config.toml")
	os.WriteFile(mainFile, []byte("main"), 0644)
	CopyGameConfigs(module)
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Fatalf("stale game config was not removed: %v", err)
	}
	if _, err := os.Stat(mainFile); err != nil {
		t.Fatalf("main config should be preserved: %v", err)
	}
}

func TestSyncDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "inner.txt"), []byte("inner"), 0644)
	dst := filepath.Join(dir, "dst")
	os.MkdirAll(filepath.Join(dst, "runtime"), 0755)
	os.WriteFile(filepath.Join(dst, "keep.txt"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("stale"), 0644)
	os.WriteFile(filepath.Join(dst, "runtime", "data.txt"), []byte("data"), 0644)
	if err := SyncDir(src, dst, "runtime"); err != nil {
		t.Fatalf("SyncDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "keep.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("keep.txt not refreshed: %q, err %v", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "inner.txt")); err != nil {
		t.Fatalf("nested file not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file was not removed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "runtime", "data.txt")); err != nil || string(data) != "data" {
		t.Fatalf("preserved dir content changed: %q, err %v", string(data), err)
	}
}
