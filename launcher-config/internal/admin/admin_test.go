package admin

import (
	"crypto/x509"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonIpc "github.com/luskaner/ageServer/launcher-common/ipc"
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
	// The CI runner may be elevated; the polling behaviour under test only
	// applies when the caller is not.
	d.isAdmin = func() bool { return false }
	return newAdmin(d)
}

func TestRunSetUpCertParseFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.bytesToCertificate = func([]byte) *x509.Certificate { return nil }
	_, exitCode := a.RunSetUp("age2", "", net.ParseIP("127.0.0.1"), false, []byte("bad"), false, false, false)
	if exitCode != internal.ErrUserCertAddParse {
		t.Fatalf("exitCode = %d, want %d", exitCode, internal.ErrUserCertAddParse)
	}
}

func TestRunSetUpNewFileFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.newFile = func(string, string, bool) (error, *commonLogger.Root) {
		return errors.New("new file fail"), nil
	}
	_, exitCode := a.RunSetUp("age2", "/tmp/log", nil, false, nil, true, false, false)
	if exitCode != common.ErrFileLog {
		t.Fatalf("exitCode = %d, want ErrFileLog %d", exitCode, common.ErrFileLog)
	}
}

func TestRunSetUpSuccessNoLog(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.runSetUp = func(string, net.IP, bool, bool, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunSetUp("age2", "", net.ParseIP("127.0.0.1"), false, nil, true, false, false)
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
	a.deps.runSetUp = func(string, net.IP, bool, bool, bool, bool, *x509.Certificate, string, io.Writer, func(*exec.Options)) *exec.Result {
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	_, exitCode := a.RunSetUp("age2", tmp, net.ParseIP("127.0.0.1"), false, nil, true, false, false)
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
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
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
	defer func() { _ = c2.Close() }()
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
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

// The host-set halves have to reach the agent unchanged: a SetupCommand that
// dropped Api made the API pass map the whole game at the API address, and
// until RootOnly existed there was no way to run the server's pass without
// overwriting the API domains back.
func TestRunSetUpAgentForwardsApiAndRootOnly(t *testing.T) {
	a := newTestAdmin(t)
	client, server := net.Pipe()
	defer func() {
		_ = client.Close()
		_ = server.Close()
	}()
	a.ipc = client
	a.enc = gob.NewEncoder(client)
	a.dec = gob.NewDecoder(client)
	var got commonIpc.SetupCommand
	// One agent goroutine serves both passes: a gob stream carries the type
	// definition of a struct only once, so a second fresh decoder on the same
	// connection would fail to read the second SetupCommand and strand the
	// test waiting for an exit code that never arrives.
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		dec := gob.NewDecoder(server)
		enc := gob.NewEncoder(server)
		for _, want := range []struct{ api, rootOnly bool }{{true, false}, {false, true}} {
			var action byte
			if err := dec.Decode(&action); err != nil {
				return
			}
			_ = enc.Encode(common.ErrSuccess) // acknowledge the command type
			// gob omits zero-valued fields on the wire, so decoding into the
			// previous pass's value would leak its true flags into this one.
			got = commonIpc.SetupCommand{}
			if err := dec.Decode(&got); err != nil {
				return
			}
			_ = enc.Encode(common.ErrSuccess) // the setup's exit code
			if want.api != got.Api || want.rootOnly != got.RootOnly {
				t.Errorf("pass travelled as Api=%v RootOnly=%v, want Api=%v RootOnly=%v",
					got.Api, got.RootOnly, want.api, want.rootOnly)
			}
		}
	}()
	err, exitCode := a.RunSetUp("age2", "", net.ParseIP("1.2.3.4"), false, nil, true, true, false)
	if err != nil || exitCode != common.ErrSuccess {
		t.Fatalf("RunSetUp api pass: err=%v exitCode=%d, want nil/%d", err, exitCode, common.ErrSuccess)
	}
	err, exitCode = a.RunSetUp("age2", "", net.ParseIP("1.2.3.5"), false, nil, true, false, true)
	if err != nil || exitCode != common.ErrSuccess {
		t.Fatalf("RunSetUp root pass: err=%v exitCode=%d, want nil/%d", err, exitCode, common.ErrSuccess)
	}
	select {
	case <-agentDone:
	case <-time.After(time.Second):
		t.Fatal("agent goroutine did not finish")
	}
}
