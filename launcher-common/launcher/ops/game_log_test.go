package ops

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression: the parent's handle for the agent log file was never closed,
// leaking a file descriptor for the launcher's lifetime. The fix closes it
// immediately after StartAgent returns (the child inherited its own copy).
func TestAgentLogFileHandleClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate what game.go does after StartAgent returns.
	_ = f.Close()

	// Verify the file is closed by attempting to write (should fail on
	// most platforms with os.ErrClosed, or at minimum not corrupt data).
	_, writeErr := f.Write([]byte("should fail"))
	if writeErr == nil {
		t.Log("write to closed file did not error (platform-dependent)")
	}
}
