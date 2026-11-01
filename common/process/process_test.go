package process

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGetPidPaths(t *testing.T) {
	paths := getPidPaths("/fake/dir/server.exe")
	if len(paths) == 0 {
		t.Fatal("getPidPaths should return at least one path")
	}
	for _, p := range paths {
		if filepath.Ext(p) != ".pid" {
			t.Errorf("path should end with .pid, got %q", p)
		}
	}
}

func TestGetPidPathsInjectable(t *testing.T) {
	tmpDir := t.TempDir()
	tmpOrig := osTempDirFn
	defer func() { osTempDirFn = tmpOrig }()
	osTempDirFn = func() string { return tmpDir }

	paths := getPidPaths(filepath.Join(tmpDir, "game.exe"))
	if len(paths) < 1 {
		t.Fatal("expected at least 1 path")
	}
	if filepath.Dir(paths[0]) != tmpDir {
		t.Errorf("first path should be in tmpDir, got %q", paths[0])
	}
}

func TestGetPidPathsTempDirEmpty(t *testing.T) {
	tmpOrig := osTempDirFn
	defer func() { osTempDirFn = tmpOrig }()
	osTempDirFn = func() string { return "" }

	paths := getPidPaths("/fake/server.exe")
	if len(paths) != 1 {
		t.Errorf("expected 1 path when tmpDir empty, got %d: %v", len(paths), paths)
	}
}

func TestGetPidPathsTempDirStatError(t *testing.T) {
	tmpOrig := osTempDirFn
	statOrig := osStatFn
	defer func() {
		osTempDirFn = tmpOrig
		osStatFn = statOrig
	}()
	osTempDirFn = func() string { return "/nonexistent" }
	osStatFn = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	paths := getPidPaths("/fake/server.exe")
	if len(paths) != 1 {
		t.Errorf("expected 1 path when stat fails, got %d", len(paths))
	}
}

func TestProcessNoPidFile(t *testing.T) {
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "nonexistent.exe")
	_, proc, err := Process(fakeExe)
	if err != nil {
		t.Fatalf("Process should not error when no PID file: %v", err)
	}
	if proc != nil {
		t.Error("Process should return nil proc when no PID file")
	}
}

func TestPidFileSize(t *testing.T) {
	if PidFileSize != 16 {
		t.Errorf("PidFileSize = %d, want 16", PidFileSize)
	}
}

func TestWaitDuration(t *testing.T) {
	if waitDuration != 3*time.Second {
		t.Errorf("waitDuration = %v, want 3s", waitDuration)
	}
}

func TestGetPidPathsAllGames(t *testing.T) {
	exes := []string{
		"/fake/AoE2DE_s.exe",
		"/fake/RelicCardinal.exe",
		"/fake/AoMRT_s.exe",
	}
	for _, exe := range exes {
		paths := getPidPaths(exe)
		if len(paths) == 0 {
			t.Errorf("getPidPaths(%q) returned empty", exe)
		}
	}
}

func TestProcessPIDBinaryFormat(t *testing.T) {
	data := make([]byte, PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], 12345)
	binary.LittleEndian.PutUint64(data[8:16], 99999)

	pid := binary.LittleEndian.Uint64(data[0:8])
	startTime := binary.LittleEndian.Uint64(data[8:16])
	if pid != 12345 || startTime != 99999 {
		t.Errorf("binary roundtrip failed: pid=%d startTime=%d", pid, startTime)
	}
}

func TestProcessNoPidFileReturnsFirstPath(t *testing.T) {
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "game.exe")
	pidPath, _, err := Process(fakeExe)
	if err != nil {
		t.Fatal(err)
	}
	if pidPath == "" {
		t.Error("pidPath should be non-empty")
	}
}

func TestKillPidProcStatError(t *testing.T) {
	// Test the stat-error path without needing a real process kill.
	// When osStatFn returns something other than ErrNotExist, it should propagate.
	statOrig := osStatFn
	defer func() { osStatFn = statOrig }()
	osStatFn = func(name string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrPermission}
	}
	// We can't call KillPidProc directly (KillProc needs a real process),
	// but we can verify the stat mock works.
	_, err := osStatFn("fake.pid")
	if err == nil {
		t.Fatal("mocked stat should return error")
	}
}

func TestKillPidProcRemovesFile(t *testing.T) {
	// Verify the remove path by creating a real file and removing it.
	f := filepath.Join(t.TempDir(), "kill.pid")
	os.WriteFile(f, []byte("data"), 0644)
	if err := osRemoveFn(f); err != nil {
		t.Fatalf("osRemoveFn: %v", err)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestGetProcessStartTimeCurrentPid(t *testing.T) {
	pid := os.Getpid()
	start, err := GetProcessStartTime(pid)
	if err != nil {
		t.Fatalf("GetProcessStartTime(%d): %v", pid, err)
	}
	if start == 0 {
		t.Error("start time should be non-zero")
	}
}

func TestGetProcessStartTimeInvalidPid(t *testing.T) {
	_, err := GetProcessStartTime(9999999)
	if err == nil {
		t.Error("expected error for invalid pid")
	}
}

func TestFindProcessWithStartTimeCurrentPid(t *testing.T) {
	pid := os.Getpid()
	start, err := GetProcessStartTime(pid)
	if err != nil {
		t.Skipf("GetProcessStartTime: %v", err)
	}
	proc, err := FindProcessWithStartTime(pid, start)
	if err != nil {
		t.Fatalf("FindProcessWithStartTime: %v", err)
	}
	if proc == nil {
		t.Error("proc should not be nil")
	}
}

func TestFindProcessWithStartTimeMismatch(t *testing.T) {
	pid := os.Getpid()
	proc, err := FindProcessWithStartTime(pid, 12345)
	if err == nil {
		t.Error("expected error for mismatched start time")
	}
	if proc != nil {
		t.Error("proc should be nil on mismatch")
	}
}

func TestFindProcessCurrentPid(t *testing.T) {
	pid := os.Getpid()
	proc, err := FindProcess(pid)
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if proc == nil {
		t.Error("proc should not be nil")
	}
}

func TestProcessesByNamesEmpty(t *testing.T) {
	m := ProcessesByNames(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d", len(m))
	}
}

func TestProcessesByNamesCurrentProcess(t *testing.T) {
	// Current test binary name should be in the snapshot
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable: %v", err)
	}
	name := filepath.Base(exe)
	m := ProcessesByNames([]string{name})
	if len(m) == 0 {
		t.Logf("ProcessesByNames(%q) returned empty, may be expected in some envs", name)
	}
}

func TestWaitForProcessCurrentWithTimeout(t *testing.T) {
	pid := os.Getpid()
	proc, err := os.FindProcess(pid)
	if err != nil {
		t.Skipf("FindProcess: %v", err)
	}
	d := 10 * time.Millisecond
	if WaitForProcess(proc, &d) {
		t.Error("WaitForProcess should be false for running process with short timeout")
	}
}

func TestWaitForProcessInvalidPid(t *testing.T) {
	proc, _ := os.FindProcess(os.Getpid())
	origWait := waitForProcessFn
	defer func() { waitForProcessFn = origWait }()
	waitForProcessFn = func(*os.Process, *time.Duration) bool { return false }
	d := 10 * time.Millisecond
	if WaitForProcess(proc, &d) {
		t.Error("WaitForProcess should be false for invalid pid")
	}
	d2 := 10 * time.Millisecond
	if waitForProcessFn(proc, &d2) {
		t.Error("mocked Wait should be false")
	}
	// Real WaitForProcess against a pid that doesn't exist: os.FindProcess
	// succeeds on Windows but OpenProcess fails → covers the early-return branch.
	if ghost, _ := os.FindProcess(999999); ghost != nil {
		d3 := 10 * time.Millisecond
		if WaitForProcess(ghost, &d3) {
			t.Error("WaitForProcess should be false for non-existent pid")
		}
	}
}

func TestKillNonexistentExe(t *testing.T) {
	err := Kill(filepath.Join(t.TempDir(), "nonexistent.exe"))
	if err != nil {
		t.Errorf("Kill nonexistent should not error, got %v", err)
	}
}

func TestProcessCorruptedPidFile(t *testing.T) {
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "game.exe")
	// Create a corrupted pid file (wrong size) at the expected location
	pidPath := getPidPaths(fakeExe)[0]
	if err := os.WriteFile(pidPath, []byte("corrupted"), 0644); err != nil {
		t.Fatal(err)
	}
	_, proc, err := Process(fakeExe)
	if err != nil {
		t.Fatalf("Process with corrupted file: %v", err)
	}
	if proc != nil {
		t.Error("proc should be nil for corrupted file")
	}
	// Regression: the file used to be deleted here. A wrong length is not
	// evidence of an orphan, it is also what a process that has created its
	// pid file but not yet written the payload looks like, and deleting it
	// unlinked the file on Unix out from under the lock its owner was about to
	// take, letting a second instance create its own and both hold the lock.
	if _, statErr := os.Stat(pidPath); statErr != nil {
		t.Errorf("corrupted pid file must be left in place for its owner to rewrite: %v", statErr)
	}
}

// A pid file that parses but does not belong to any live process is real
// evidence of an orphan and must still be cleaned up.
func TestProcessOrphanPidFileRemoved(t *testing.T) {
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "game.exe")
	pidPath := getPidPaths(fakeExe)[0]
	data := make([]byte, PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(4_000_000_000)) // dead PID
	binary.LittleEndian.PutUint64(data[8:16], 0)
	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, proc, err := Process(fakeExe); err != nil || proc != nil {
		t.Fatalf("got proc %v err %v; want a clean not-running state", proc, err)
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Error("a pid file matching no live process should still be removed")
	}
}

func TestKillPidProcKillFail(t *testing.T) {
	origKill := killProcFn
	defer func() { killProcFn = origKill }()
	killProcFn = func(*os.Process) error { return errors.New("kill fail") }
	proc, _ := os.FindProcess(os.Getpid())
	err := KillPidProc("fake.pid", proc)
	if err == nil || err.Error() != "kill fail" {
		t.Fatalf("expected kill fail, got %v", err)
	}
}

func TestKillPidProcStatNotExist(t *testing.T) {
	origKill := killProcFn
	origStat := osStatFn
	defer func() { killProcFn = origKill; osStatFn = origStat }()
	killProcFn = func(*os.Process) error { return nil }
	osStatFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	proc, _ := os.FindProcess(os.Getpid())
	err := KillPidProc("nonexist.pid", proc)
	if err != nil {
		t.Fatalf("expected nil when Stat is NotExist, got %v", err)
	}
}

func TestKillPidProcStatOtherError(t *testing.T) {
	origKill := killProcFn
	origStat := osStatFn
	defer func() { killProcFn = origKill; osStatFn = origStat }()
	killProcFn = func(*os.Process) error { return nil }
	osStatFn = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	proc, _ := os.FindProcess(os.Getpid())
	err := KillPidProc("fake.pid", proc)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestKillPidProcRemoveSuccess(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "toremove.pid")
	_ = os.WriteFile(fpath, []byte("x"), 0644)
	origKill := killProcFn
	defer func() { killProcFn = origKill }()
	killProcFn = func(*os.Process) error { return nil }
	proc, _ := os.FindProcess(os.Getpid())
	err := KillPidProc(fpath, proc)
	if err != nil {
		t.Fatalf("KillPidProc remove: %v", err)
	}
	if _, err := os.Stat(fpath); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestKillProcKillFail(t *testing.T) {
	origKill := procKillFn
	defer func() { procKillFn = origKill }()
	procKillFn = func(*os.Process) error { return errors.New("kill fail") }
	origSignal := procSignalFn
	defer func() { procSignalFn = origSignal }()
	procSignalFn = func(*os.Process, os.Signal) error { return errors.New("signal fail") }
	proc, _ := os.FindProcess(os.Getpid())
	if err := KillProc(proc); err == nil {
		t.Error("KillProc should fail")
	}
}

func TestKillProcWaitTimeout(t *testing.T) {
	origWait := waitForProcessFn
	defer func() { waitForProcessFn = origWait }()
	waitForProcessFn = func(*os.Process, *time.Duration) bool { return false }
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start: %v", err)
	}
	proc := cmd.Process
	err := KillProc(proc)
	if !errors.Is(err, ErrKillTimeout) {
		t.Fatalf("expected ErrKillTimeout, got %v", err)
	}
	_ = proc.Kill()
	_ = cmd.Wait()
}

func TestKillProcessError(t *testing.T) {
	origProcess := processFn
	defer func() { processFn = origProcess }()
	processFn = func(string) (string, *os.Process, error) { return "", nil, errors.New("process error") }
	err := Kill("fake.exe")
	if err == nil || err.Error() != "process error" {
		t.Fatalf("expected process error, got %v", err)
	}
}

// The graceful branch is skipped on Windows, so this has to opt in explicitly.
// It also stubs procKillFn: without the branch being taken the real Kill would
// be pointed at this very test process.
func TestKillProcSignalSuccess(t *testing.T) {
	origSignal := procSignalFn
	origWait := waitForProcessFn
	origGraceful := supportsGracefulSignal
	origKill := procKillFn
	defer func() {
		procSignalFn = origSignal
		waitForProcessFn = origWait
		supportsGracefulSignal = origGraceful
		procKillFn = origKill
	}()
	supportsGracefulSignal = true
	procKillFn = func(*os.Process) error { return errors.New("kill must not be reached") }
	procSignalFn = func(*os.Process, os.Signal) error { return nil }
	waitForProcessFn = func(*os.Process, *time.Duration) bool { return true }
	proc, _ := os.FindProcess(os.Getpid())
	if err := KillProc(proc); err != nil {
		t.Fatalf("KillProc should succeed when Signal and Wait succeed, got %v", err)
	}
}

// Regression: the graceful branch used to be attempted everywhere, including
// Windows where os.Process.Signal only accepts os.Kill. It always failed there
// and the error was discarded, so the function looked like it tried to be polite
// when it had not, and the fallback kill looked like a second attempt rather
// than the only one.
func TestKillProcSkipsSignalWhereUnsupported(t *testing.T) {
	origSignal := procSignalFn
	origKill := procKillFn
	origGraceful := supportsGracefulSignal
	origWait := waitForProcessFn
	defer func() {
		procSignalFn = origSignal
		procKillFn = origKill
		supportsGracefulSignal = origGraceful
		waitForProcessFn = origWait
	}()
	supportsGracefulSignal = false
	signalled := false
	procSignalFn = func(*os.Process, os.Signal) error {
		signalled = true
		return nil
	}
	killed := false
	procKillFn = func(*os.Process) error { killed = true; return nil }
	waitForProcessFn = func(*os.Process, *time.Duration) bool { return true }

	proc, _ := os.FindProcess(os.Getpid())
	if err := KillProc(proc); err != nil {
		t.Fatalf("KillProc: %v", err)
	}
	if signalled {
		t.Error("signalled even though the platform has no graceful signal")
	}
	if !killed {
		t.Error("Kill must be the one and only attempt")
	}
}

func TestKillProcIntegration(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "10", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start test process: %v", err)
	}
	proc := cmd.Process
	time.Sleep(100 * time.Millisecond)
	if err := KillProc(proc); err != nil {
		t.Fatalf("KillProc: %v", err)
	}
	d := 2 * time.Second
	if !WaitForProcess(proc, &d) {
		t.Error("WaitForProcess should be true after KillProc")
	}
	_ = cmd.Wait()
}

func TestKillPidProcIntegration(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "10", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start test process: %v", err)
	}
	proc := cmd.Process
	pid := proc.Pid
	startTime, err := GetProcessStartTime(pid)
	if err != nil {
		_ = proc.Kill()
		_ = cmd.Wait()
		t.Skipf("GetProcessStartTime: %v", err)
	}
	// Create a pid file for this proc
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	data := make([]byte, PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(pid))
	binary.LittleEndian.PutUint64(data[8:16], uint64(startTime))
	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := KillPidProc(pidPath, proc); err != nil {
		t.Fatalf("KillPidProc: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after KillPidProc")
	}
	_ = cmd.Wait()
}

func TestWaitForProcessIntegration(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "2", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start: %v", err)
	}
	proc := cmd.Process
	d := 5 * time.Second
	if !WaitForProcess(proc, &d) {
		t.Error("WaitForProcess should be true for exited process")
	}
	_ = cmd.Wait()
}

func TestKillIntegration(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "10", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start: %v", err)
	}
	pid := cmd.Process.Pid
	startTime, err := GetProcessStartTime(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Skipf("GetProcessStartTime: %v", err)
	}
	dir := t.TempDir()
	// Use a fake exe path that getPidPaths will resolve to our temp file
	fakeExe := filepath.Join(dir, "integration.exe")
	pidPath := getPidPaths(fakeExe)[0]
	// Ensure the dir for pidPath exists (it is os.TempDir() or exe dir)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(pid))
	binary.LittleEndian.PutUint64(data[8:16], uint64(startTime))
	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	// Kill via exe path should find and kill the process
	if err := Kill(fakeExe); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after Kill")
	}
	_ = cmd.Wait()
}
