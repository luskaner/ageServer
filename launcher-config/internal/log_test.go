package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression: Initialize discarded the NewFile error, leaving Logger nil and
// making every subsequent Buffer() call a silent no-op (logs lost).
func TestInitializeInvalidRootReturnsError(t *testing.T) {
	old := Logger
	defer func() { Logger = old }()

	fileParent := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(fileParent, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	badRoot := filepath.Join(fileParent, "logs")

	if err := Initialize(badRoot); err == nil {
		t.Fatal("expected error for unusable log root")
	}
	if Logger != nil {
		t.Fatal("Logger must be nil after failed initialization")
	}
}

func TestInitializeValidRootSetsLogger(t *testing.T) {
	old := Logger
	defer func() { Logger = old }()

	root := t.TempDir()
	if err := Initialize(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Logger == nil {
		t.Fatal("Logger must be set after successful initialization")
	}
}
