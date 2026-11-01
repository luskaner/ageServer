package admin

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The fallback kill cannot work from an unelevated process against an elevated
// agent on Windows: TerminateProcess on a process at a higher integrity level is
// denied. This pins that StopAgentIfNeeded reports the failure rather than
// claiming success, which is what leaves a caller believing the cleanup is done.
func TestStopAgentReportsAccessDeniedInsteadOfSuccess(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("not connected") }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	alive := true
	a.deps.process = func(string) (string, *os.Process, error) {
		if alive {
			return "/tmp/pid", &os.Process{Pid: 1}, nil
		}
		return "", nil, nil
	}
	killed := false
	a.deps.killPidProc = func(string, *os.Process) error {
		killed = true
		return windows.ERROR_ACCESS_DENIED
	}
	a.deps.sleep = func(time.Duration) {}

	if a.StopAgentIfNeeded() {
		t.Fatal("StopAgentIfNeeded reported success while the agent was still running")
	}
	if !killed {
		t.Error("the kill fallback should still have been attempted")
	}
}

// Once the kill does succeed the function must say so, otherwise a working
// elevated launcher would still look like a failure.
func TestStopAgentReportsSuccessfulKill(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("not connected") }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	alive := true
	a.deps.process = func(string) (string, *os.Process, error) {
		if alive {
			return "/tmp/pid", &os.Process{Pid: 1}, nil
		}
		return "", nil, nil
	}
	a.deps.killPidProc = func(string, *os.Process) error {
		alive = false
		return nil
	}
	a.deps.sleep = func(time.Duration) {}

	if !a.StopAgentIfNeeded() {
		t.Fatal("expected true after the kill succeeded")
	}
}

func TestIsAccessDeniedWindows(t *testing.T) {
	if !isAccessDenied(windows.ERROR_ACCESS_DENIED) {
		t.Error("ERROR_ACCESS_DENIED should be recognised")
	}
	if !isAccessDenied(os.ErrPermission) {
		t.Error("os.ErrPermission should be recognised")
	}
	if isAccessDenied(nil) {
		t.Error("nil is not an access denial")
	}
	if isAccessDenied(errors.New("something else")) {
		t.Error("an unrelated error is not an access denial")
	}
}
