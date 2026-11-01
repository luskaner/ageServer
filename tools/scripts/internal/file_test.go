package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCpSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst", "dst.txt")
	os.WriteFile(src, []byte("hello"), 0644)
	if err := Cp(src, dst); err != nil {
		t.Fatalf("Cp failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "hello" {
		t.Fatalf("dst content %q err %v", string(data), err)
	}
}

func TestCpSrcNotExist(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "nonexistent.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := Cp(src, dst); err == nil {
		t.Fatal("should fail when src not exist")
	}
}

func TestCpDstIsDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("x"), 0644)
	dstDir := filepath.Join(dir, "dstDir")
	os.Mkdir(dstDir, 0755)
	// Try to copy to a path where dst is a directory (Create will fail because dst is directory)
	dst := filepath.Join(dstDir, "subdir")
	os.Mkdir(dst, 0755)
	// Now dst is a directory, Cp will try to MkdirP for dst's dir (which is dstDir) and then Create(dst) where dst is a directory -> should fail
	if err := Cp(src, dst); err == nil {
		t.Fatal("should fail when dst is directory")
	}
}

func TestMkdirPSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c")
	MkdirP(path)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("MkdirP failed: %v", err)
	}
	// Calling again should not panic
	MkdirP(path)
}

func TestCopyMainConfig(t *testing.T) {
	// Create a temp module structure
	tmp := t.TempDir()
	// Need to create the source file at <tmp>/<module>/resources/config.toml
	// But CopyMainConfig uses ResourcePath which is <module>/resources/config.toml relative to current working dir
	// So we need to chdir to tmp and create the structure there
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)
	module := "testmod"
	srcDir := filepath.Join(tmp, module, "resources")
	os.MkdirAll(srcDir, 0755)
	srcFile := filepath.Join(srcDir, "config.toml")
	os.WriteFile(srcFile, []byte("test"), 0644)
	// Also need to ensure BuildDir exists: build/<module>/resources
	// CopyMainConfig will call Cp which will create dst dir
	CopyMainConfig(module)
	dstFile := filepath.Join(tmp, "build", module, "resources", "config.toml")
	if _, err := os.Stat(dstFile); err != nil {
		t.Fatalf("dst not created: %v", err)
	}
	data, _ := os.ReadFile(dstFile)
	if string(data) != "test" {
		t.Fatalf("content %q", string(data))
	}
}

func TestCopyGameConfigs(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)
	module := "testmod2"
	srcDir := filepath.Join(tmp, module, "resources")
	os.MkdirAll(srcDir, 0755)
	// Create source game config template
	srcFile := filepath.Join(srcDir, "config.game.toml")
	os.WriteFile(srcFile, []byte("game"), 0644)
	CopyGameConfigs(module)
	// Check that at least one game config was copied
	dstFile := filepath.Join(tmp, "build", module, "resources", "config.age1.toml")
	if _, err := os.Stat(dstFile); err != nil {
		t.Fatalf("game config not copied: %v", err)
	}
}
