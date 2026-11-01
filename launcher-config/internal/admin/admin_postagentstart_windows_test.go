package admin

import (
	"errors"
	"net"
	"testing"
	"time"
)

// Readiness means the agent's IPC endpoint answers. It used to mean its pid file
// existed on Unix and was not checked at all on Windows, so the two platforms
// disagreed about the one question that decides whether the agent is usable, and
// the pid file is written before the agent can serve anything.
func TestPostAgentStartProbesThePipe(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return &probeConn{}, nil }
	if !a.postAgentStart() {
		t.Fatal("an agent whose endpoint answers must be reported as ready")
	}
}

func TestPostAgentStartWaitsForThePipe(t *testing.T) {
	a := newTestAdmin(t)
	calls := 0
	a.deps.dialIPC = func() (net.Conn, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("not listening yet")
		}
		return &probeConn{}, nil
	}
	a.deps.sleep = func(time.Duration) {}
	if !a.postAgentStart() {
		t.Fatal("expected success once the endpoint answers")
	}
	if calls != 3 {
		t.Errorf("probed %d times, want 3", calls)
	}
}

// Regression: the old loop was unbounded with no failure path, so an agent that
// never came up left config.exe hanging forever with nothing printed.
func TestPostAgentStartGivesUpWhenThePipeNeverAnswers(t *testing.T) {
	a := newTestAdmin(t)
	a.deps.dialIPC = func() (net.Conn, error) { return nil, errors.New("no agent") }
	polls := 0
	a.deps.sleep = func(time.Duration) { polls++ }

	if a.postAgentStart() {
		t.Fatal("expected failure when the agent never answers")
	}
	if polls != postAgentStartAttempts {
		t.Errorf("polled %d times, want the whole budget of %d", polls, postAgentStartAttempts)
	}
	if postAgentStartAttempts <= 0 {
		t.Error("postAgentStartAttempts must be positive for the loop to progress")
	}
}

// probeConn is a net.Conn that records nothing; postAgentStart only closes it.
type probeConn struct{}

func (probeConn) Read([]byte) (int, error)         { return 0, nil }
func (probeConn) Write([]byte) (int, error)        { return 0, nil }
func (probeConn) Close() error                     { return nil }
func (probeConn) LocalAddr() net.Addr              { return nil }
func (probeConn) RemoteAddr() net.Addr             { return nil }
func (probeConn) SetDeadline(time.Time) error      { return nil }
func (probeConn) SetReadDeadline(time.Time) error  { return nil }
func (probeConn) SetWriteDeadline(time.Time) error { return nil }
