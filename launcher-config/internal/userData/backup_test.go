package userData

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
)

// Exercises the full switchPaths cycle through the exported API: Backup moves
// active -> .bak leaving .lan in place; Restore moves it back.
func TestBackupRestoreCycle(t *testing.T) {
	base := t.TempDir()
	active := filepath.Join(base, "profile")

	d := Data{Path: active}
	if err := os.MkdirAll(active, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "save.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if !d.Backup() {
		t.Fatal("Backup failed")
	}
	// Original data must live in the .bak directory...
	bakData, err := os.ReadFile(filepath.Join(base, "profile.bak", "save.txt"))
	if err != nil || string(bakData) != "data" {
		t.Fatalf(".bak content = %q err = %v", bakData, err)
	}
	// ...and the active path must be a fresh EMPTY directory for the game.
	entries, err := os.ReadDir(active)
	if err != nil || len(entries) != 0 {
		t.Fatalf("active dir after Backup = %v (%d entries), want empty", entries, len(entries))
	}

	if !d.Restore() {
		t.Fatal("Restore failed")
	}
	restored, err := os.ReadFile(filepath.Join(active, "save.txt"))
	if err != nil || string(restored) != "data" {
		t.Fatalf("restored content = %q err = %v", restored, err)
	}
	if _, err = os.Stat(filepath.Join(base, "profile.bak")); !os.IsNotExist(err) {
		t.Fatal(".bak must be consumed by Restore")
	}
}

func TestBackupWhenOriginalMissingCreatesHierarchy(t *testing.T) {
	base := t.TempDir()
	// Deep hierarchy whose parents do not exist yet.
	active := filepath.Join(base, "Games", "My Games", "Age of Empires 2 DE")
	d := Data{Path: active}

	if !d.Backup() {
		t.Fatal("Backup failed")
	}
	// The whole hierarchy must exist with a fresh active dir, and the
	// original data preserved in the .bak sibling.
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active dir missing: %v", err)
	}
	if _, err := os.Stat(active + ".bak"); err != nil {
		t.Fatalf(".bak sibling missing: %v", err)
	}
	_ = commonUserData.TypeActive
}

// A rename that cannot happen must say why. Before this, switchPaths printed
// "Renaming/Moving" and then returned false with no further line, so a profile
// folder held open by another process (Explorer sitting on it, the indexer, the
// antivirus) looked like a hang and the log never named the path at fault.
func TestLogSwitchFailureNamesPathAndReason(t *testing.T) {
	var buf bytes.Buffer
	commonLogger.Initialize(&buf)
	defer commonLogger.Initialize(os.Stdout)

	logSwitchFailure("rename", `C:\p\1`, fs.ErrPermission)

	out := buf.String()
	if !strings.Contains(out, `C:\p\1`) {
		t.Fatalf("output does not name the path: %q", out)
	}
	if !strings.Contains(out, fs.ErrPermission.Error()) {
		t.Fatalf("output does not carry the reason: %q", out)
	}
}

// renameWithRetry must surface the error once the attempts are exhausted, so the
// caller can log it instead of swallowing it.
func TestRenameWithRetryReturnsError(t *testing.T) {
	base := t.TempDir()
	if err := renameWithRetry(filepath.Join(base, "missing"), filepath.Join(base, "target")); err == nil {
		t.Fatal("expected an error renaming a missing path")
	}
}

// And it must still rename when the first attempt is enough.
func TestRenameWithRetrySucceeds(t *testing.T) {
	base := t.TempDir()
	from := filepath.Join(base, "from")
	if err := os.MkdirAll(from, 0755); err != nil {
		t.Fatal(err)
	}
	to := filepath.Join(base, "to")
	if err := renameWithRetry(from, to); err != nil {
		t.Fatalf("renameWithRetry: %v", err)
	}
	if _, err := os.Stat(to); err != nil {
		t.Fatalf("target missing: %v", err)
	}
}
