package ipc

import (
	"encoding/gob"
	"net"
	"testing"
	"time"

	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal"
)

// Regression: after an undecodable action the loop used to `continue`, and
// since the gob stream state is corrupt it would block/error forever instead
// of dropping the malicious or broken client.
func TestHandleClientReturnsOnUndecodableAction(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	done := make(chan bool, 1)
	go func() {
		handleClient("", server)
		done <- true
	}()

	// A complete gob frame with an unexpected type fails Decode immediately
	// (type mismatch, not EOF) after being fully consumed.
	if err := gob.NewEncoder(client).Encode("not-an-action"); err != nil {
		t.Fatal(err)
	}

	// The agent must answer with ErrDecode before dropping the connection.
	_ = client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var replyCode int
	if err := gob.NewDecoder(client).Decode(&replyCode); err != nil {
		t.Fatalf("no ErrDecode response: %v", err)
	}
	if replyCode != internal.ErrDecode {
		t.Fatalf("reply = %d, want ErrDecode (%d)", replyCode, internal.ErrDecode)
	}

	select {
	case <-done:
		// Expected: connection dropped right after answering ErrDecode.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handleClient did not return after an undecodable action (infinite error loop)")
	}
}

func TestHandleClientEOFExitsCleanly(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()

	done := make(chan bool, 1)
	go func() {
		handleClient("", server)
		done <- true
	}()

	_ = client.Close()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handleClient did not return on EOF")
	}
}
