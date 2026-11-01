package admin

import (
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-config/internal"
)

// newTestAdmin returns an Admin with production deps except that sleep is a
// no-op and the logger is cleared, so each test runs isolated from globals.
func newTestAdmin(t *testing.T) *Admin {
	t.Helper()
	oldLogger := internal.Logger
	t.Cleanup(func() { internal.Logger = oldLogger })
	internal.Logger = nil
	d := defaultDeps()
	d.sleep = func(time.Duration) {}
	return newAdmin(d)
}

func TestRunSetUpCertParseFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.bytesToCertificate = func([]byte) *x509.Certificate { return nil }
	_, exitCode := a.RunSetUp("age2", "", net.ParseIP("127.0.0.1"), false, []byte("bad"), false)
	if exitCode != internal.ErrUserCertAddParse {
		t.Fatalf("exitCode = %d, want %d", exitCode, internal.ErrUserCertAddParse)
	}
}

func TestRunSetUpNewFileFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.newFile = func(string, string, bool) (error, *commonLogger.Root) {
		return errors.New("new file fail"), nil
	}
	_, exitCode := a.RunSetUp("age2", "/tmp/log", nil, false, nil, true)
	if exitCode != common.ErrFileLog {
		t.Fatalf("exitCode = %d, want ErrFileLog %d", exitCode, common.ErrFileLog)
	}
}

func TestRunSetUpSuccessNoLog(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.runSetUp = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunSetUp("age2", "", net.ParseIP("127.0.0.1"), false, nil, true)
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunSetUpSuccessWithLog(t *testing.T) {
	a := newTestAdmin(t)
	tmp := t.TempDir()
	a.deps.newFile = func(root, gameId string, finalRoot bool) (error, *commonLogger.Root) {
		return commonLogger.NewFile(tmp, "", true)
	}
	a.deps.runSetUp = func(string, net.IP, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunSetUp("age2", tmp, net.ParseIP("127.0.0.1"), false, nil, true)
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunRevertNewFileFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.newFile = func(string, string, bool) (error, *commonLogger.Root) {
		return errors.New("fail"), nil
	}
	_, exitCode := a.RunRevert("/tmp/log", true, true, true)
	if exitCode != common.ErrFileLog {
		t.Fatalf("exitCode = %d, want ErrFileLog", exitCode)
	}
}

func TestRunRevertSuccessNoLog(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.runRevert = func(bool, bool, bool, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunRevert("", true, false, true)
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunFlushCacheAgentAlreadyStarted(t *testing.T) {
	a := newTestAdmin(t)
	a.ipc = &mockConn{}
	_, exitCode := a.RunFlushCache("", true, true)
	if exitCode != internal.ErrAgentAlreadyStarted {
		t.Fatalf("exitCode = %d, want ErrAgentAlreadyStarted %d", exitCode, internal.ErrAgentAlreadyStarted)
	}
}

func TestRunFlushCacheNewFileFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.newFile = func(string, string, bool) (error, *commonLogger.Root) {
		return errors.New("fail"), nil
	}
	_, exitCode := a.RunFlushCache("/tmp/log", true, true)
	if exitCode != common.ErrFileLog {
		t.Fatalf("exitCode = %d, want ErrFileLog", exitCode)
	}
}

func TestRunFlushCacheSuccessNoLog(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.runFlushCache = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		return "", &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunFlushCache("", true, true)
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestConnectAgentAlreadyConnected(t *testing.T) {
	a := newTestAdmin(t)
	a.ipc = &mockConn{}
	if err := a.ConnectAgentIfNeeded(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestConnectAgentDialFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("dial fail") }
	if err := a.ConnectAgentIfNeeded(); err == nil {
		t.Fatal("expected error")
	}
}

func TestConnectAgentDialSuccess(t *testing.T) {
	a := newTestAdmin(t)
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	a.deps.dialIPC = func() (net.Conn, error) { return c1, nil }
	if err := a.ConnectAgentIfNeeded(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if a.ipc != c1 {
		t.Fatal("ipc not set")
	}
	if a.enc == nil || a.dec == nil {
		t.Fatal("encoder/decoder not set")
	}
}

func TestStopAgentWhenNotConnectedAndNoProcess(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("not connected") }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	a.deps.process = func(string) (string, *os.Process, error) { return "", nil, nil }
	if !a.StopAgentIfNeeded() {
		t.Fatal("expected true when no process")
	}
}

func TestStopAgentKillSuccess(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("not") }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	a.deps.process = func(string) (string, *os.Process, error) {
		return "/tmp/pid", &os.Process{Pid: 1}, nil
	}
	a.deps.killPidProc = func(string, *os.Process) error { return nil }
	if !a.StopAgentIfNeeded() {
		t.Fatal("expected true after kill")
	}
}

func TestStartAgentSuccess(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.getLoggerFolder = func() string { return "" }
	a.deps.runFlushCacheAgent = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		return "/tmp/file", &exec.Result{ExitCode: common.ErrSuccess, Pid: 123}
	}
	a.deps.dialIPC = func() (net.Conn, error) { return nil, nil }
	result := a.StartAgent(true, true)
	if !result.Success() {
		t.Fatalf("expected success, got %+v", result)
	}
}

func TestStartAgentPostStartFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.getLoggerFolder = func() string { return "" }
	a.deps.runFlushCacheAgent = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		return "/tmp/file", &exec.Result{ExitCode: common.ErrSuccess, Pid: 123}
	}
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("no agent") }
	a.deps.sleep = func(time.Duration) {}
	result := a.StartAgent(true, true)
	if result.Err == nil {
		t.Fatal("expected error when postAgentStart fails")
	}
}

func TestStartAgentFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.getLoggerFolder = func() string { return "" }
	a.deps.runFlushCacheAgent = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		return "", &exec.Result{Err: errors.New("start fail"), ExitCode: 1}
	}
	result := a.StartAgent(true, true)
	if result.Success() {
		t.Fatal("expected failure")
	}
}

func TestConnectWithRetriesSuccess(t *testing.T) {
	a := newTestAdmin(t)
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	a.deps.dialIPC = func() (net.Conn, error) { return c1, nil }
	if !a.ConnectAgentIfNeededWithRetries() {
		t.Fatal("expected true")
	}
}

func TestConnectWithRetriesFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("fail") }
	if a.ConnectAgentIfNeededWithRetries() {
		t.Fatal("expected false")
	}
}

func TestClearIPCState(t *testing.T) {
	a := newTestAdmin(t)
	c1, c2 := net.Pipe()
	defer c2.Close()
	a.ipc = c1
	a.enc = nil
	a.dec = nil
	a.clearIPCState()
	if a.ipc != nil {
		t.Fatal("ipc should be nil")
	}
}

// mockConn implements net.Conn for tests
type mockConn struct{ net.Conn }

func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
