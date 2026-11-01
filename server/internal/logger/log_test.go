package logger

import (
	"os"
	"path/filepath"
	"testing"

	commonLogger "github.com/luskaner/ageServer/common/logger"
)

func TestOpenMainFileLogDisabled(t *testing.T) {
	// When logEnabled false, should not create logger and return nil
	commonLogger.FileLogger = nil
	if err := OpenMainFileLog("/tmp", false); err != nil {
		t.Fatalf("should not error when disabled, got %v", err)
	}
	if commonLogger.FileLogger != nil {
		t.Fatal("FileLogger should remain nil when disabled")
	}
}

func TestOpenMainFileLogEnabledSuccess(t *testing.T) {
	dir := t.TempDir()
	commonLogger.FileLogger = nil
	t.Cleanup(func() { commonLogger.FileLogger = nil })
	if err := OpenMainFileLog(dir, true); err != nil {
		t.Fatalf("should succeed, got %v", err)
	}
	if commonLogger.FileLogger == nil {
		t.Fatal("FileLogger should be set")
	}
	// Cleanup file
	commonLogger.CloseFileLog()
}

func TestOpenMainFileLogEnabledFailure(t *testing.T) {
	// Make a file where directory should be, to cause MkdirAll failure
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	os.WriteFile(blocker, []byte("x"), 0644)
	badRoot := filepath.Join(blocker, "logs")
	commonLogger.FileLogger = nil
	t.Cleanup(func() { commonLogger.FileLogger = nil })
	if err := OpenMainFileLog(badRoot, true); err == nil {
		t.Fatal("should fail with bad root")
	}
}

func TestPrintFileNoLogger(t *testing.T) {
	commonLogger.FileLogger = nil
	// Should not panic when logger nil
	PrintFile("test", "/tmp/somefile")
}

func TestPrintFileWithLogger(t *testing.T) {
	dir := t.TempDir()
	commonLogger.FileLogger = nil
	t.Cleanup(func() { commonLogger.FileLogger = nil; commonLogger.CloseFileLog() })
	OpenMainFileLog(dir, true)
	// Create a temp file to print
	tmpFile := filepath.Join(dir, "sample.txt")
	os.WriteFile(tmpFile, []byte("hello"), 0644)
	PrintFile("sample", tmpFile)
	// Should not panic, and should have logged
}

func TestPrintfAndPrintln(t *testing.T) {
	// These just call commonLogger and fmt, should not panic
	commonLogger.Initialize(nil)
	Printf("test %d", 42)
	Println("hello", "world")
}
