package cmd

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal"
)

// Regression: the agent used to create its named pipe only after flushing the
// cache, and the flush runs ipconfig, which takes seconds on a slow machine.
// The caller polls for the pipe for a couple of seconds after launching the
// agent, gave up, and killed an agent that was merely still starting. The pid
// file is written before either, so the caller could see it alive throughout.
// The agent then finished flushing, began accepting, and sat there forever.
//
// The pipe has to exist before the slow work, not after it.
func TestRunRootListensBeforeFlushing(t *testing.T) {
	resetState(t)
	values.IPs = true

	var order []string
	listenFn = func() (net.Listener, error) {
		order = append(order, "listen")
		return fakeListener{}, nil
	}
	runFlushCacheFn = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		order = append(order, "flush")
		return "", &exec.Result{ExitCode: common.ErrSuccess}
	}
	serveFn = func(string, net.Listener) int {
		order = append(order, "serve")
		return common.ErrSuccess
	}

	if _, code := runRoot(nil); code != common.ErrSuccess {
		t.Fatalf("code=%d, want success", code)
	}
	want := []string{"listen", "flush", "serve"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}
}

// Serving still has to wait for the flush, otherwise a client could be handed a
// command while the cache is being dropped underneath it.
func TestRunRootServesAfterFlushNotBefore(t *testing.T) {
	resetState(t)
	values.IPs = true
	servedDuringFlush := false
	inFlush := false

	listenFn = func() (net.Listener, error) { return fakeListener{}, nil }
	runFlushCacheFn = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		inFlush = true
		defer func() { inFlush = false }()
		time.Sleep(20 * time.Millisecond)
		return "", &exec.Result{ExitCode: common.ErrSuccess}
	}
	serveFn = func(string, net.Listener) int {
		if inFlush {
			servedDuringFlush = true
		}
		return common.ErrSuccess
	}

	if _, code := runRoot(nil); code != common.ErrSuccess {
		t.Fatalf("code=%d, want success", code)
	}
	if servedDuringFlush {
		t.Error("Serve ran while the cache flush was still going")
	}
}

// A failure to listen must be reported and must not go on to flush or serve.
func TestRunRootListenFailure(t *testing.T) {
	resetState(t)
	values.IPs = true
	flushed := false
	listenFn = func() (net.Listener, error) { return nil, errors.New("no pipe") }
	runFlushCacheFn = func(bool, bool, string, io.Writer, func(*exec.Options)) (string, *exec.Result) {
		flushed = true
		return "", &exec.Result{ExitCode: common.ErrSuccess}
	}
	serveFn = func(string, net.Listener) int {
		t.Error("Serve must not run when the pipe could not be created")
		return common.ErrSuccess
	}

	if _, code := runRoot(nil); code != internal.ErrListen {
		t.Fatalf("code=%d, want ErrListen %d", code, internal.ErrListen)
	}
	if flushed {
		t.Error("flush must not run when the pipe could not be created")
	}
}
