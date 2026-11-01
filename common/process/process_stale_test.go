package process

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// Regression: a stale (orphan) pid file used to leave a residual error
// (e.g. ESRCH) in the named return, so callers saw err != nil for the normal
// "nothing is running" case.
func TestProcessStalePidFileReturnsCleanState(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "ghost-process.exe")
	pidPaths := getPidPaths(exe)
	if len(pidPaths) == 0 {
		t.Fatal("getPidPaths returned no candidates")
	}
	stale := pidPaths[0]
	t.Cleanup(func() {
		for _, p := range pidPaths {
			_ = os.Remove(p)
		}
	})

	data := make([]byte, PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(4_000_000_000)) // dead PID
	binary.LittleEndian.PutUint64(data[8:16], 0)
	if err := os.WriteFile(stale, data, 0644); err != nil {
		t.Fatal(err)
	}

	pidPath, proc, err := Process(exe)
	if err != nil {
		t.Fatalf("stale pid file must not produce an error, got %v", err)
	}
	if proc != nil {
		t.Fatal("no live process expected")
	}
	if pidPath != stale {
		t.Fatalf("pidPath = %q, want first candidate %q", pidPath, stale)
	}
	if _, statErr := os.Stat(stale); !os.IsNotExist(statErr) {
		t.Fatal("stale pid file must be removed as orphan")
	}
}

func TestProcessNoPidFileAtAll(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "never-ran.exe")
	for _, p := range getPidPaths(exe) {
		_ = os.Remove(p)
	}
	pidPath, proc, err := Process(exe)
	if err != nil || proc != nil {
		t.Fatalf("got %v, %v; want clean not-running state", err, proc)
	}
	if pidPath == "" {
		t.Fatal("pidPath must still report where the lock would live")
	}
}

func TestGetPidPathsIncludesTempAndExeDir(t *testing.T) {
	exe := filepath.Join("some", "dir", "tool.exe")
	paths := getPidPaths(exe)
	wantTemp := filepath.Join(os.TempDir(), common.Name+"-tool.exe.pid")
	found := false
	for _, p := range paths {
		if p == wantTemp {
			found = true
		}
	}
	if !found {
		t.Fatalf("paths %v missing temp candidate %q", paths, wantTemp)
	}
	last := paths[len(paths)-1]
	// The candidate is anchored to the target's real directory, so a relative
	// exe path is resolved against the cwd first rather than kept as a
	// cwd-relative fragment. See getPidPaths.
	wantDir := filepath.Dir(exe)
	if abs, err := filepath.Abs(exe); err == nil {
		wantDir = filepath.Dir(abs)
	}
	if last != filepath.Join(wantDir, common.Name+"-tool.exe.pid") {
		t.Fatalf("exe-dir candidate = %q, want %q", last, filepath.Join(wantDir, common.Name+"-tool.exe.pid"))
	}
}

// Regression: the two callers that decide whether 'config-admin-agent' is
// running spelled its path differently, and only agreed by accident. Every
// binary chdirs to its own directory (common.ChdirToExe), so config.exe passes
// the bare "config-admin-agent.exe" while the launcher, one level up, passes
// "bin\config-admin-agent.exe". The temp candidate is keyed on the base name and
// masked the difference; with the temp location unavailable the exe-dir
// candidates diverged to bin\ and bin\bin\, so each side read a pid file the
// other never wrote and concluded nothing was running.
func TestGetPidPathsSpellingIndependent(t *testing.T) {
	tmpOrig := osTempDirFn
	statOrig := osStatFn
	absOrig := filepathAbsFn
	defer func() {
		osTempDirFn = tmpOrig
		osStatFn = statOrig
		filepathAbsFn = absOrig
	}()

	// Drop the temp candidate so only the exe-dir one is left to compare.
	osTempDirFn = func() string { return "/nonexistent" }
	osStatFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	// Build the layout with filepath so the test asserts the same thing on
	// Windows and Unix. A hardcoded "C:\..." literal is a single path component
	// on Unix, which silently turns this into a test of separator handling.
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	want := filepath.Join(binDir, common.Name+"-config-admin-agent.exe.pid")

	// The two callers run from different directories, which is the whole reason
	// they used to disagree: mirror filepath.Abs against a cwd we control.
	cwd := ""
	filepathAbsFn = func(p string) (string, error) {
		if filepath.IsAbs(p) {
			return p, nil
		}
		return filepath.Join(cwd, p), nil
	}

	// config.exe chdir'd into bin/ and passes the bare name.
	cwd = binDir
	fromBin := getPidPaths(`config-admin-agent.exe`)
	if len(fromBin) != 1 {
		t.Fatalf("expected only the exe-dir candidate, got %v", fromBin)
	}
	if got := fromBin[0]; got != want {
		t.Errorf("bare name from bin/ resolved to %q, want %q", got, want)
	}

	// The launcher chdir'd into bin's parent and passes bin\<name>.
	cwd = root
	fromParent := getPidPaths(filepath.Join("bin", "config-admin-agent.exe"))
	if len(fromParent) != 1 {
		t.Fatalf("expected only the exe-dir candidate, got %v", fromParent)
	}
	if got := fromParent[0]; got != want {
		t.Errorf("bin-prefixed name from bin's parent resolved to %q, want %q", got, want)
	}
}

// An unresolvable path must not collapse the candidate list or panic; the
// cwd-relative result is the best we can do and is better than no candidate.
func TestGetPidPathsAbsFailureFallsBack(t *testing.T) {
	absOrig := filepathAbsFn
	tmpOrig := osTempDirFn
	statOrig := osStatFn
	defer func() {
		filepathAbsFn = absOrig
		osTempDirFn = tmpOrig
		osStatFn = statOrig
	}()
	osTempDirFn = func() string { return "/nonexistent" }
	osStatFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	filepathAbsFn = func(string) (string, error) { return "", errors.New("no cwd") }

	paths := getPidPaths(filepath.Join("bin", "tool.exe"))
	if len(paths) != 1 {
		t.Fatalf("expected 1 candidate, got %v", paths)
	}
	if got, want := paths[0], filepath.Join("bin", common.Name+"-tool.exe.pid"); got != want {
		t.Errorf("candidate = %q, want the cwd-relative %q", got, want)
	}
}
