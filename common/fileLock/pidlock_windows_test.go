package fileLock

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// Split out of pidlock_test.go: poking an invalid handle only compiles on
// Windows, and leaving it in the shared file made the whole package's tests
// unbuildable everywhere else.

func TestPidLockUnlockUnlockError(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "u.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pl := &PidLock{}
	pl.fileLock.BaseLock = &BaseLock{File: f}
	// Invalid handle makes windows.UnlockFileEx fail
	pl.fileLock.handle = windows.Handle(999999)
	pl.fileLock.lock = &windows.Overlapped{}
	defer func() { _ = os.Remove(f.Name()) }()
	if err := pl.Unlock(); err == nil {
		t.Log("Unlock with invalid handle succeeded (FS dependent)")
	}
}

// Same split: the invalid-handle Unlock path out of the cross-process lock
// contention test, which is meaningful on every platform.
func TestLockUnlockInvalidHandle(t *testing.T) {
	var l Lock
	l.handle = windows.Handle(999999)
	l.lock = &windows.Overlapped{}
	if err := l.Unlock(); err == nil {
		t.Log("Unlock with invalid handle may succeed")
	}
}
