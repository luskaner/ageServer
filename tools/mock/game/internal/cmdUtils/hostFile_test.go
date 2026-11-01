package cmdUtils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandleHostFileSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	// Create a simple hosts file with one line
	content := "127.0.0.1 myhost.example.com\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := HandleHostFile(path); err != nil {
		t.Fatalf("should succeed, got %v", err)
	}
}

func TestHandleHostFileNotExist(t *testing.T) {
	if err := HandleHostFile("/nonexistent/path/hosts"); err == nil {
		t.Fatal("should fail when file not exist")
	}
}

func TestHandleHostFileInvalidContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	// Write invalid content that GetAllLines will fail? Actually GetAllLines may succeed with empty, but we can test with a file that is not readable?
	// Create a file with binary content that will cause GetAllLines to error due to encoding?
	// For now, just test that it handles a file with a valid line that is not an IP (should still succeed, just no caching)
	os.WriteFile(path, []byte("not an ip line\n"), 0644)
	if err := HandleHostFile(path); err != nil {
		t.Fatalf("should succeed even with invalid line, got %v", err)
	}
}
