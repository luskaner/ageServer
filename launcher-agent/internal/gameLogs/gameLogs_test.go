package gameLogs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyPathToDirFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "log.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	if !copyPathToDir(src, dstDir) {
		t.Fatal("copy reported failure")
	}
	data, err := os.ReadFile(filepath.Join(dstDir, "log.txt"))
	if err != nil || string(data) != "content" {
		t.Fatalf("copied content = %q err = %v", data, err)
	}
}

func TestCopyPathToDirDirectoryTree(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	tree := filepath.Join(srcDir, "session")
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "root.txt"), []byte("r"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "sub", "leaf.txt"), []byte("l"), 0644); err != nil {
		t.Fatal(err)
	}

	if !copyPathToDir(tree, dstDir) {
		t.Fatal("copy reported failure")
	}
	for _, rel := range []string{"root.txt", filepath.Join("sub", "leaf.txt")} {
		data, err := os.ReadFile(filepath.Join(dstDir, "session", rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s empty", rel)
		}
	}
}

func TestCopyPathToDirMissingSourceFails(t *testing.T) {
	if copyPathToDir(filepath.Join(t.TempDir(), "nope"), t.TempDir()) {
		t.Fatal("missing source must fail")
	}
}

func TestAddNewestPathPicksMostRecent(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "old.txt")
	newer := filepath.Join(base, "new.txt")
	if err := os.WriteFile(old, []byte("o"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Force distinct modification times (filesystems have 1s-2s granularity).
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(old, future, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	var paths []string
	addNewestPath(base, []string{old, newer}, func(info os.FileInfo) bool { return !info.IsDir() }, &paths)

	if len(paths) != 1 || paths[0] != newer {
		t.Fatalf("paths = %v, want newest %q", paths, newer)
	}
}

func TestCopyGameLogsUnknownGameIsNoop(t *testing.T) {
	CopyGameLogs("not-a-game", t.TempDir(), t.TempDir()) // must not panic
}

func TestSortByModTime(t *testing.T) {
	dir := t.TempDir()
	files := make([]os.FileInfo, 3)
	names := []string{"c.txt", "a.txt", "b.txt"}
	for i, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		// Stagger mod times
		if err := os.Chtimes(path, time.Now().Add(time.Duration(i)*time.Hour), time.Now().Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		files[i] = info
	}
	sortByModTime(&files)
	// Should be sorted newest first (i=2, i=1, i=0) => b, a, c
	if files[0].Name() != "b.txt" || files[1].Name() != "a.txt" || files[2].Name() != "c.txt" {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.Name()
		}
		t.Fatalf("sortByModTime order = %v, want [b a c]", names)
	}
}

func TestAddNewestPathNoMatchingFiles(t *testing.T) {
	var paths []string
	addNewestPath(t.TempDir(), []string{"/nonexistent"}, func(info os.FileInfo) bool { return true }, &paths)
	if len(paths) != 0 {
		t.Fatalf("expected empty paths, got %v", paths)
	}
}

func TestAddNewestPathFiltersDirectories(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "subdir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file.txt")
	if err := os.WriteFile(file, []byte("f"), 0644); err != nil {
		t.Fatal(err)
	}
	// checkFn rejects directories
	var paths []string
	addNewestPath(base, []string{dir, file}, func(info os.FileInfo) bool { return !info.IsDir() }, &paths)
	if len(paths) != 1 || paths[0] != file {
		t.Fatalf("paths = %v, want [%s]", paths, file)
	}
}

func TestCopyFileContentSuccess(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.txt")
	dst := filepath.Join(t.TempDir(), "dst.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if !copyFileContent(src, dst) {
		t.Fatal("copyFileContent returned false")
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "hello" {
		t.Fatalf("content = %q, err = %v", data, err)
	}
}

func TestCopyFileContentSourceMissing(t *testing.T) {
	if copyFileContent("/nonexistent", filepath.Join(t.TempDir(), "dst")) {
		t.Fatal("should fail for missing source")
	}
}

func TestCopyPathToDirMkdirAllFails(t *testing.T) {
	src := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// dst is a file, not a directory — MkdirAll will fail
	dst := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dst, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if copyPathToDir(src, dst) {
		t.Fatal("should fail when dst is a file")
	}
}
