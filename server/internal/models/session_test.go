package models

import (
	"testing"
	"time"

	"github.com/luskaner/ageServer/server/internal"
)

func TestAddMessageNonBlockingWhenFull(t *testing.T) {
	sess := &SessionData{
		messageChan: make(chan internal.A, 2),
	}

	// Fill the buffer.
	for i := 0; i < 2; i++ {
		sess.AddMessage(internal.A{int32(i)})
	}

	// This must NOT block even though the channel is full.
	done := make(chan struct{})
	go func() {
		sess.AddMessage(internal.A{99})
		close(done)
	}()

	select {
	case <-done:
		// Non-blocking send completed (message was dropped).
	case <-time.After(time.Second):
		t.Fatal("AddMessage blocked on full channel — goroutine leak")
	}
}

func TestWaitForMessagesDrainsChannel(t *testing.T) {
	sess := &SessionData{
		messageChan: make(chan internal.A, 10),
	}
	expected := []internal.A{{1}, {2}, {3}}
	for _, msg := range expected {
		sess.AddMessage(msg)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Simulate the timer firing to end WaitForMessages.
	}()

	ackNum, results := sess.WaitForMessages(0)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if ackNum != 1 {
		t.Fatalf("ackNum = %d, want 1 (one batch read)", ackNum)
	}
}
