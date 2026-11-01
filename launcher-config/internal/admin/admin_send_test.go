package admin

import (
	"bytes"
	"encoding/gob"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	commonIpc "github.com/luskaner/ageServer/launcher-common/ipc"
)

// failingWriter fails on every Write.
type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (int, error) { return 0, errors.New("write fail") }

// failingReader fails on Read and trivially satisfies net.Conn.
type failingReader struct{}

func (f *failingReader) Read(p []byte) (int, error) { return 0, errors.New("read fail") }
func (f *failingReader) Write(p []byte) (int, error) {
	return len(p), nil
}
func (f *failingReader) Close() error                       { return nil }
func (f *failingReader) LocalAddr() net.Addr                { return nil }
func (f *failingReader) RemoteAddr() net.Addr               { return nil }
func (f *failingReader) SetDeadline(t time.Time) error      { return nil }
func (f *failingReader) SetReadDeadline(t time.Time) error  { return nil }
func (f *failingReader) SetWriteDeadline(t time.Time) error { return nil }

// statefulFailWriter fails on the failure-th Write call.
type statefulFailWriter struct {
	count  int
	failOn int
	buf    bytes.Buffer
}

func (s *statefulFailWriter) Write(p []byte) (int, error) {
	s.count++
	if s.count == s.failOn {
		return 0, errors.New("write fail")
	}
	return s.buf.Write(p)
}

// agentConn pairs an Admin with the far end of a net.Pipe, mirroring a real
// config-admin-agent IPC connection. serve funnels agent-side errors back to
// the test and waitServer joins the goroutine before returning.
type agentConn struct {
	a        *Admin
	local    net.Conn
	remote   net.Conn
	enc      *gob.Encoder
	dec      *gob.Decoder
	serverWg sync.WaitGroup
}

func newAgentConn(t *testing.T) *agentConn {
	t.Helper()
	a := newTestAdmin(t)
	local, remote := net.Pipe()
	ac := &agentConn{
		a:      a,
		local:  local,
		remote: remote,
	}
	ac.enc = gob.NewEncoder(local)
	ac.dec = gob.NewDecoder(local)
	a.ipc = local
	a.enc = ac.enc
	a.dec = ac.dec
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})
	return ac
}

// serve runs fn on the remote (agent) side and reports any error on the
// returned channel.
func (ac *agentConn) serve(fn func(enc *gob.Encoder, dec *gob.Decoder) error) <-chan error {
	errc := make(chan error, 1)
	ac.serverWg.Add(1)
	go func() {
		defer ac.serverWg.Done()
		defer close(errc)
		errc <- fn(gob.NewEncoder(ac.remote), gob.NewDecoder(ac.remote))
	}()
	return errc
}

// waitServer waits for the agent goroutine and reports any agent-side errors as
// test failures.
func (ac *agentConn) waitServer(t *testing.T, errc <-chan error) {
	t.Helper()
	ac.serverWg.Wait()
	for err := range errc {
		if err != nil {
			t.Errorf("agent side error: %v", err)
		}
	}
}

func TestSendAgentSetupSuccessViaPipe(t *testing.T) {
	ac := newAgentConn(t)
	errc := ac.serve(func(enc *gob.Encoder, dec *gob.Decoder) error {
		var typ byte
		if err := dec.Decode(&typ); err != nil {
			return err
		}
		if err := enc.Encode(common.ErrSuccess); err != nil {
			return err
		}
		if typ == commonIpc.Setup {
			var cmd commonIpc.SetupCommand
			if err := dec.Decode(&cmd); err != nil {
				return err
			}
		} else {
			var cmd commonIpc.RevertCommand
			if err := dec.Decode(&cmd); err != nil {
				return err
			}
		}
		return enc.Encode(common.ErrSuccess)
	})

	err, code := ac.a.runSetUpAgent("age2", net.ParseIP("127.0.0.1"), false, false, []byte("cert"))
	ac.waitServer(t, errc)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if code != common.ErrSuccess {
		t.Fatalf("expected success code 0, got %d", code)
	}
}

func TestSendAgentRevertSuccessViaPipe(t *testing.T) {
	ac := newAgentConn(t)
	errc := ac.serve(func(enc *gob.Encoder, dec *gob.Decoder) error {
		var typ byte
		if err := dec.Decode(&typ); err != nil {
			return err
		}
		if typ != commonIpc.Revert {
			return errors.New("expected Revert command type")
		}
		if err := enc.Encode(common.ErrSuccess); err != nil {
			return err
		}
		var cmd commonIpc.RevertCommand
		if err := dec.Decode(&cmd); err != nil {
			return err
		}
		return enc.Encode(common.ErrSuccess)
	})

	err, code := ac.a.runRevertAgent(true, false)
	ac.waitServer(t, errc)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if code != common.ErrSuccess {
		t.Fatalf("code = %d, want 0", code)
	}
}

func TestSendAgentEncodeTypeFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.ipc = &mockConn{}
	fw := &failingWriter{}
	a.enc = gob.NewEncoder(fw)
	a.dec = gob.NewDecoder(bytes.NewReader(nil))
	err, _ := a.sendAgent(commonIpc.Setup, "Setup", func() any { return commonIpc.SetupCommand{} })
	if err == nil {
		t.Fatal("expected encode error")
	}
}

func TestSendAgentDecodeFirstExitFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.ipc = &mockConn{}
	var buf bytes.Buffer
	a.enc = gob.NewEncoder(&buf)
	a.dec = gob.NewDecoder(&failingReader{})

	err, _ := a.sendAgent(commonIpc.Setup, "Setup", func() any { return commonIpc.SetupCommand{} })
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestSendAgentFirstDecodeNonSuccessExit(t *testing.T) {
	ac := newAgentConn(t)
	errc := ac.serve(func(enc *gob.Encoder, dec *gob.Decoder) error {
		var typ byte
		if err := dec.Decode(&typ); err != nil {
			return err
		}
		return enc.Encode(int(1)) // non-success exit code
	})

	_, code := ac.a.runSetUpAgent("age2", nil, false, true, nil)
	ac.waitServer(t, errc)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestSendAgentEncodeDataFailure(t *testing.T) {
	a := newTestAdmin(t)
	a.ipc = &mockConn{}
	w := &statefulFailWriter{failOn: 2}
	a.enc = gob.NewEncoder(w)
	var buf bytes.Buffer
	_ = gob.NewEncoder(&buf).Encode(common.ErrSuccess)
	a.dec = gob.NewDecoder(&buf)
	err, _ := a.sendAgent(commonIpc.Setup, "Setup", func() any { return commonIpc.SetupCommand{} })
	if err == nil {
		t.Fatal("expected encode failure on data write")
	}
}

func TestSendAgentDecodeFinalFailure(t *testing.T) {
	ac := newAgentConn(t)
	errc := ac.serve(func(enc *gob.Encoder, dec *gob.Decoder) error {
		var typ byte
		if err := dec.Decode(&typ); err != nil {
			return err
		}
		if err := enc.Encode(common.ErrSuccess); err != nil {
			return err
		}
		var cmd commonIpc.SetupCommand
		if err := dec.Decode(&cmd); err != nil {
			return err
		}
		// Close the pipe so the client's final exit-code decode fails.
		return ac.remote.Close()
	})

	err, _ := ac.a.runSetUpAgent("age2", nil, false, true, nil)
	ac.waitServer(t, errc)
	if err == nil {
		t.Fatal("expected error decoding final exit code after agent closed connection")
	}
}

func TestRunSetUpAgentViaSendAgentSuccess(t *testing.T) {
	ac := newAgentConn(t)
	errc := ac.serve(func(enc *gob.Encoder, dec *gob.Decoder) error {
		var typ byte
		if err := dec.Decode(&typ); err != nil {
			return err
		}
		if typ != commonIpc.Setup {
			return errors.New("expected Setup type")
		}
		if err := enc.Encode(common.ErrSuccess); err != nil {
			return err
		}
		var cmd commonIpc.SetupCommand
		if err := dec.Decode(&cmd); err != nil {
			return err
		}
		if cmd.GameId != "age3" {
			return errors.New("unexpected game id")
		}
		if cmd.IP.String() != "1.2.3.4" {
			return errors.New("unexpected ip")
		}
		if !cmd.MacOsExclusiveMappings {
			return errors.New("expected MacOsExclusiveMappings true")
		}
		if !cmd.CanUseInternet {
			return errors.New("expected CanUseInternet true")
		}
		return enc.Encode(common.ErrSuccess)
	})

	err, code := ac.a.runSetUpAgent("age3", net.ParseIP("1.2.3.4"), true, true, []byte("certdata"))
	ac.waitServer(t, errc)
	if err != nil || code != common.ErrSuccess {
		t.Fatalf("expected success, got err %v code %d", err, code)
	}
}

func TestStopAgentIfNeededWithIPC(t *testing.T) {
	a := newTestAdmin(t)
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	a.ipc = local
	a.enc = gob.NewEncoder(local)
	a.dec = gob.NewDecoder(local)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dec := gob.NewDecoder(remote)
		var cmd byte
		_ = dec.Decode(&cmd)
		// The agent acknowledges the shutdown before closing the connection.
		_ = gob.NewEncoder(remote).Encode(common.ErrSuccess)
	}()
	a.deps.process = func(string) (string, *os.Process, error) { return "", nil, nil }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	if !a.StopAgentIfNeeded() {
		t.Fatal("expected true when stopping via IPC with no process")
	}
	if a.ipc != nil {
		t.Fatal("ipc should be cleared after stop")
	}
	wg.Wait()
}

// Regression: we cannot terminate the elevated agent from a non-elevated
// process, so a missing acknowledgement means the shutdown command never
// landed. It must be reported rather than silently treated as "sent".
func TestStopAgentIfNeededReportsMissingAck(t *testing.T) {
	a := newTestAdmin(t)
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	a.ipc = local
	a.enc = gob.NewEncoder(local)
	a.dec = gob.NewDecoder(local)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dec := gob.NewDecoder(remote)
		var cmd byte
		_ = dec.Decode(&cmd)
		// Drop the connection instead of acknowledging.
		_ = remote.Close()
	}()
	// The process never goes away, so the poll times out and the kill is tried.
	a.deps.process = func(string) (string, *os.Process, error) { return "/tmp/pid", &os.Process{Pid: 1}, nil }
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	a.deps.killPidProc = func(string, *os.Process) error { return os.ErrPermission }
	a.deps.sleep = func(time.Duration) {}
	if a.StopAgentIfNeeded() {
		t.Fatal("expected false: the agent neither acknowledged nor could be killed")
	}
	if a.ipc != nil {
		t.Error("ipc should be cleared even when the acknowledgement is missing")
	}
	wg.Wait()
}

// A slow agent must not be declared failed: the acknowledgement plus the poll
// has to be given room to complete the shutdown.
func TestStopAgentIfNeededWaitsForSlowAgent(t *testing.T) {
	a := newTestAdmin(t)
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	a.ipc = local
	a.enc = gob.NewEncoder(local)
	a.dec = gob.NewDecoder(local)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dec := gob.NewDecoder(remote)
		var cmd byte
		_ = dec.Decode(&cmd)
		_ = gob.NewEncoder(remote).Encode(common.ErrSuccess)
	}()

	// Still running for the first few polls, then gone.
	polls := 0
	a.deps.process = func(string) (string, *os.Process, error) {
		polls++
		if polls > 5 {
			return "", nil, nil
		}
		return "/tmp/pid", &os.Process{Pid: 1}, nil
	}
	a.deps.nativeFileName = func(bool, string) string { return "dummy.exe" }
	a.deps.killPidProc = func(string, *os.Process) error { t.Error("should not need to kill an acknowledged agent"); return nil }
	a.deps.sleep = func(time.Duration) {}
	if !a.StopAgentIfNeeded() {
		t.Fatal("expected true: a slow but acknowledged agent must still succeed")
	}
	if polls <= 5 {
		t.Errorf("polls = %d, expected the poll to keep waiting while the agent was alive", polls)
	}
	wg.Wait()
}
