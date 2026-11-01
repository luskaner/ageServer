package fileLock

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/process"
)

// Regression: openFile used to return (nil, nil) when another instance was
// running, and Lock(nil) produced confusing OS errors. It must now surface
// ErrAlreadyRunning.
func TestOpenFileAlreadyRunningSelf(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot resolve executable")
	}
	name := common.Name + "-" + filepath.Base(exe) + ".pid"
	pidPath := filepath.Join(os.TempDir(), name)
	t.Cleanup(func() { _ = os.Remove(pidPath) })

	startTime, startErr := process.GetProcessStartTime(os.Getpid())
	if startErr != nil {
		// Without a real start time we cannot build a self-describing pid file.
		t.Skipf("GetProcessStartTime unavailable: %v", startErr)
	}
	data := make([]byte, process.PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(os.Getpid()))
	binary.LittleEndian.PutUint64(data[8:16], uint64(startTime))
	if err = os.WriteFile(pidPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	openErr, f := openFile()
	if !errors.Is(openErr, ErrAlreadyRunning) {
		t.Fatalf("openFile err = %v, want ErrAlreadyRunning", openErr)
	}
	if f != nil {
		t.Fatal("no file must be returned when already running")
	}

	lockErr := (&PidLock{}).Lock()
	if !errors.Is(lockErr, ErrAlreadyRunning) {
		t.Fatalf("Lock err = %v, want ErrAlreadyRunning", lockErr)
	}
}

func TestPidLockFullCycleAndIdempotentUnlock(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "unit.pid")

	f, err := os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	pl := &PidLock{}
	if err = pl.fileLock.Lock(f); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err = writePid(f); err != nil {
		t.Fatalf("writePid: %v", err)
	}

	// Contents round-trip.
	data, err := os.ReadFile(pidPath)
	if err != nil || len(data) != process.PidFileSize {
		t.Fatalf("pid file len = %d err = %v", len(data), err)
	}
	if got := binary.LittleEndian.Uint64(data[0:8]); int(got) != os.Getpid() {
		t.Fatalf("stored pid = %d", got)
	}

	if err = pl.Unlock(); err != nil {
		t.Fatalf("first Unlock: %v", err)
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Fatal("pid file must be removed on Unlock")
	}
	// Regression: a second Unlock used to operate on cleaned-up state.
	if err = pl.Unlock(); err != nil {
		t.Fatalf("second Unlock must be safe and nil, got %v", err)
	}
}

func TestNewBaseLock(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "lock.dat"))
	if err != nil {
		t.Fatal(err)
	}
	bl := NewBaseLock(f)
	if bl == nil || bl.File != f {
		t.Fatal("NewBaseLock must return BaseLock with the given file")
	}
	bl.close()
	if bl.File != nil {
		t.Error("close() should set File to nil")
	}
}

func TestBaseLockCloseNilFile(t *testing.T) {
	bl := &BaseLock{}
	// Should not panic when File is already nil
	bl.close()
}

func TestLockContended(t *testing.T) {
	dir := t.TempDir()
	f1, err := os.Create(filepath.Join(dir, "c.dat"))
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	l1 := Lock{}
	if err := l1.Lock(f1); err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	l1.clean()

	// Second lock on same file via a fresh Lock — may succeed on Windows
	// (non-exclusive by default) but should not panic.
	f2, err := os.OpenFile(f1.Name(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	l2 := Lock{}
	_ = l2.Lock(f2)
	l2.clean()
}

func TestPidLockUnlockWithoutLock(t *testing.T) {
	pl := &PidLock{}
	if err := pl.Unlock(); err != nil {
		t.Errorf("Unlock without prior Lock should be no-op, got %v", err)
	}
}

func TestWritePidTruncatesFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "truncate.pid")
	f, err := os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Write extra data first
	f.Write([]byte("extra data that should be truncated"))
	f.Close()

	f, err = os.OpenFile(pidPath, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePid(f); err != nil {
		t.Fatalf("writePid: %v", err)
	}
	f.Close()

	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != process.PidFileSize {
		t.Errorf("file size = %d after writePid, want %d", len(data), process.PidFileSize)
	}
}

func TestNewBaseLockWithNilFile(t *testing.T) {
	bl := NewBaseLock(nil)
	if bl == nil {
		t.Fatal("NewBaseLock(nil) should not return nil")
	}
	if bl.File != nil {
		t.Error("File should be nil")
	}
	// close on nil file should not panic
	bl.close()
}

func TestLockAndUnlockCycle(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "cycle.dat"))
	if err != nil {
		t.Fatal(err)
	}

	l := &Lock{}
	if err := l.Lock(f); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	// File should be closed after unlock
	if l.File != nil {
		t.Error("File should be nil after Unlock")
	}
}

func TestPidLockErrAlreadyRunningIsSentinel(t *testing.T) {
	if ErrAlreadyRunning.Error() != "another instance is already running" {
		t.Errorf("ErrAlreadyRunning message = %q", ErrAlreadyRunning.Error())
	}
}

func TestWritePidClosedFile(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "closed.pid"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := writePid(f); err == nil {
		t.Error("writePid on closed file should error")
	}
}

func TestRemoveFileNotExist(t *testing.T) {
	if err := removeFile(filepath.Join(t.TempDir(), "notexist.pid")); err == nil {
		t.Error("removeFile on nonexistent should error")
	}
}

func TestOpenFileOsExecutableError(t *testing.T) {
	orig := osExecutableFn
	defer func() { osExecutableFn = orig }()
	osExecutableFn = func() (string, error) { return "", errors.New("exe fail") }
	err, f := openFile()
	if err == nil || f != nil {
		t.Fatalf("expected exe fail with nil file, got %v %v", err, f)
	}
}

func TestOpenFileProcessError(t *testing.T) {
	orig := processProcessFn
	defer func() { processProcessFn = orig }()
	processProcessFn = func(string) (string, *os.Process, error) { return "", nil, errors.New("process fail") }
	err, f := openFile()
	if err == nil || f != nil {
		t.Fatalf("expected process fail with nil file, got %v %v", err, f)
	}
}

func TestWritePidWriteError(t *testing.T) {
	origTrunc, origWrite := fileTruncateFn, fileWriteFn
	defer func() { fileTruncateFn, fileWriteFn = origTrunc, origWrite }()
	fileTruncateFn = func(*os.File, int64) error { return nil }
	fileWriteFn = func(*os.File, []byte) (int, error) { return 0, errors.New("write fail") }
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "w.pid"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := writePid(f); err == nil || err.Error() != "write fail" {
		t.Fatalf("expected write fail, got %v", err)
	}
}

func TestPidLockLockSuccess(t *testing.T) {
	// Ensure no stale pid file
	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot resolve executable")
	}
	name := common.Name + "-" + filepath.Base(exe) + ".pid"
	pidPath := filepath.Join(os.TempDir(), name)
	_ = os.Remove(pidPath)
	t.Cleanup(func() { _ = os.Remove(pidPath) })

	pl := &PidLock{}
	if err := pl.Lock(); err != nil {
		t.Fatalf("PidLock.Lock should succeed when no other instance, got %v", err)
	}
	// Verify pid file was created and contains current pid
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("pid file not created: %v", err)
	}
	if len(data) != process.PidFileSize {
		t.Fatalf("pid file size = %d, want %d", len(data), process.PidFileSize)
	}
	if got := binary.LittleEndian.Uint64(data[0:8]); int(got) != os.Getpid() {
		t.Errorf("stored pid = %d, want %d", got, os.Getpid())
	}
	if err := pl.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after Unlock")
	}
}

func TestPidLockLockOpenFileError(t *testing.T) {
	orig := openFileFn
	defer func() { openFileFn = orig }()
	openFileFn = func() (error, *os.File) { return errors.New("open fail"), nil }
	if err := (&PidLock{}).Lock(); err == nil || err.Error() != "open fail" {
		t.Fatalf("expected open fail, got %v", err)
	}
}

func TestPidLockLockFileLockError(t *testing.T) {
	origOpen := openFileFn
	defer func() { openFileFn = origOpen }()
	f, err := os.CreateTemp("", "lockfail*.pid")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// Return a closed file so Lock fails
	openFileFn = func() (error, *os.File) {
		ff, _ := os.OpenFile(f.Name(), os.O_RDWR, 0644)
		ff.Close()
		return nil, ff
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if err := (&PidLock{}).Lock(); err == nil {
		t.Error("expected fileLock.Lock error")
	}
}

func TestPidLockLockWritePidError(t *testing.T) {
	origOpen := openFileFn
	origWrite := writePidFn
	defer func() { openFileFn = origOpen; writePidFn = origWrite }()
	f, err := os.CreateTemp("", "writefail*.pid")
	if err != nil {
		t.Fatal(err)
	}
	// Keep file open for Lock to succeed
	openFileFn = func() (error, *os.File) { return nil, f }
	writePidFn = func(*os.File) error { return errors.New("write fail") }
	defer func() { _ = os.Remove(f.Name()); f.Close() }()
	if err := (&PidLock{}).Lock(); err == nil || err.Error() != "write fail" {
		t.Fatalf("expected write fail, got %v", err)
	}
	// After write failure, clean should have been called, file should be closed
	if f.Name() != "" {
		_ = os.Remove(f.Name())
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("TEST_LOCK_CHILD") == "1" {
		path := os.Getenv("TEST_LOCK_PATH")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			os.Exit(1)
		}
		var l Lock
		if err := l.Lock(f); err != nil {
			os.Exit(1)
		}
		_ = os.WriteFile(path+".locked", []byte("locked"), 0644)
		time.Sleep(5 * time.Second)
		_ = l.Unlock()
		_ = os.Remove(path + ".locked")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestLockIntegrationContended(t *testing.T) {
	if os.Getenv("TEST_LOCK_CHILD") == "1" {
		t.Skip("parent skip in child")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "contended.lock")
	// Create file so child can open it
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.count=1")
	cmd.Env = append(os.Environ(), "TEST_LOCK_CHILD=1", "TEST_LOCK_PATH="+path)
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start child: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(path + ".locked")
	}()
	// Wait for child to acquire lock (signal file)
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(path + ".locked"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	f2, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	var l Lock
	err = l.Lock(f2)
	if err == nil {
		_ = l.Unlock()
		t.Log("second lock succeeded (may be FS dependent)")
	} else {
		t.Logf("second lock failed as expected: %v", err)
	}
}
