package exec

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// Regression: SEE_MASK_NOCLOSEPROCESS makes ShellExecuteExW hand over a process
// handle, and nothing released it. A session that elevates config-admin-agent a
// few times leaked a handle each time, for the lifetime of the process. These
// pin the release down for both paths that receive a handle.

// fakeShellExecute writes handle into the SHELLEXECUTEINFO it is handed and
// reports success, so the code under test behaves as if the API had started
// something.
func fakeShellExecute(handle windows.Handle) func(info *SHELLEXECUTEINFO) (uintptr, uintptr, error) {
	return func(info *SHELLEXECUTEINFO) (uintptr, uintptr, error) {
		info.hProcess = handle
		return 1, 0, nil
	}
}

func captureClosedHandles(t *testing.T) *[]windows.Handle {
	t.Helper()
	var closed []windows.Handle
	restore := SetCloseHandleFn(func(h windows.Handle) error {
		closed = append(closed, h)
		return nil
	})
	t.Cleanup(restore)
	return &closed
}

func TestShellExecuteExClosesHandleOnPidPath(t *testing.T) {
	defer SetShellExecuteExCallFn(nil)
	restoreCall := SetShellExecuteExCallFn(fakeShellExecute(windows.Handle(4242)))
	defer restoreCall()
	closed := captureClosedHandles(t)
	origPid := getProcessIdFn
	SetGetProcessIdFn(func(windows.Handle) (uint32, error) { return 1234, nil })
	defer SetGetProcessIdFn(origPid)

	// start=true, getPid=true: the caller only wants the pid number.
	_, pid, _ := shellExecuteEx("open", true, `C:\Windows\System32\cmd.exe`, true, true, windows.SW_NORMAL, "/c", "echo", "x")
	if pid != 1234 {
		t.Fatalf("pid = %d, want 1234", pid)
	}
	if len(*closed) != 1 || (*closed)[0] != windows.Handle(4242) {
		t.Fatalf("closed = %v, want exactly [4242]", *closed)
	}
}

func TestShellExecuteExClosesHandleOnWaitPath(t *testing.T) {
	defer SetShellExecuteExCallFn(nil)
	restoreCall := SetShellExecuteExCallFn(fakeShellExecute(windows.Handle(99)))
	defer restoreCall()
	closed := captureClosedHandles(t)
	origWait := waitForSingleObjectFn
	SetWaitForSingleObjectFn(func(windows.Handle, uint32) (uint32, error) { return uint32(windows.WAIT_OBJECT_0), nil })
	defer SetWaitForSingleObjectFn(origWait)
	origExit := getExitCodeProcessFn
	SetGetExitCodeProcessFn(func(_ windows.Handle, code *uint32) error { *code = 0; return nil })
	defer SetGetExitCodeProcessFn(origExit)

	// start=false: the handle is needed for the exit code, then must be released.
	_, _, exitCode := shellExecuteEx("open", false, `C:\Windows\System32\cmd.exe`, true, false, windows.SW_NORMAL, "/c", "echo", "x")
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if len(*closed) != 1 || (*closed)[0] != windows.Handle(99) {
		t.Fatalf("closed = %v, want exactly [99]", *closed)
	}
}

// A failed ShellExecuteEx may still leave a handle behind; it must be released
// too, and CloseHandle's own error must not mask the real failure.
func TestShellExecuteExClosesHandleOnFailure(t *testing.T) {
	defer SetShellExecuteExCallFn(nil)
	restoreCall := SetShellExecuteExCallFn(func(info *SHELLEXECUTEINFO) (uintptr, uintptr, error) {
		info.hProcess = windows.Handle(7)
		return 0, 0, errors.New("shell execute failed")
	})
	defer restoreCall()
	closed := captureClosedHandles(t)

	err, pid, _ := shellExecuteEx("open", true, `C:\Windows\System32\cmd.exe`, true, true, windows.SW_NORMAL, "/c", "echo", "x")
	if err == nil {
		t.Fatal("expected the original failure to be reported")
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0 on failure", pid)
	}
	if len(*closed) != 1 || (*closed)[0] != windows.Handle(7) {
		t.Fatalf("closed = %v, want exactly [7]", *closed)
	}
}

// No handle means nothing to release, and CloseHandle must not be called with
// zero.
func TestShellExecuteExNoHandleNoClose(t *testing.T) {
	defer SetShellExecuteExCallFn(nil)
	restoreCall := SetShellExecuteExCallFn(func(*SHELLEXECUTEINFO) (uintptr, uintptr, error) { return 1, 0, nil })
	defer restoreCall()
	closed := captureClosedHandles(t)
	origPid := getProcessIdFn
	SetGetProcessIdFn(func(h windows.Handle) (uint32, error) { return 0, errors.New("no handle") })
	defer SetGetProcessIdFn(origPid)

	_, _, _ = shellExecuteEx("open", true, `C:\Windows\System32\cmd.exe`, true, true, windows.SW_NORMAL, "/c", "echo", "x")
	if len(*closed) != 0 {
		t.Fatalf("closed = %v, want none for a zero handle", *closed)
	}
}
