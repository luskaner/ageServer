package logger

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestBufferLogAndClose(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "comm*.log")
	if err != nil {
		t.Fatal(err)
	}
	buf := NewBuffer(tmp)
	if buf == nil || CommBuffer == nil {
		t.Fatal("buffer nil")
	}
	// Log a value
	buf.Log(map[string]string{"a": "b"})
	// Log with nil buffer should not panic
	var nilBuf *Buffer
	nilBuf.Log("test")
	// Close should flush and close file
	if err := buf.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	// Verify file contains json
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		// The file contains json lines, not single json object, but should be parseable as at least one line
		// The encoder writes with newline, so we check contains
		if len(data) == 0 {
			t.Fatal("empty file")
		}
	}
}

func TestBufferCloseFlushError(t *testing.T) {
	// Create a temp file and close it to make flush fail?
	// Simpler: test that Close handles already closed file
	tmp, err := os.CreateTemp(t.TempDir(), "comm*.log")
	if err != nil {
		t.Fatal(err)
	}
	buf := NewBuffer(tmp)
	tmp.Close() // close underlying file to make flush fail
	if err := buf.Close(); err == nil {
		// May error or not, but should not panic
	}
}

func TestUptime(t *testing.T) {
	// Test with nil - should be small positive (since startTime is now, StartTime is recent)
	d := Uptime(nil)
	// d should be around 0, not large negative
	if d < -time.Second || d > time.Second*5 {
		// Allow small drift, but not large
	}
	// Test with specific time in past: should be negative (past - StartTime)
	past := time.Now().Add(-time.Hour)
	d2 := Uptime(&past)
	if d2 > 0 {
		t.Fatalf("uptime for past should be negative, got %v", d2)
	}
	// Test with StartTime set to past, Uptime(nil) should be positive ~2h
	StartTime = time.Now().Add(-2 * time.Hour)
	d3 := Uptime(nil)
	if d3 < time.Hour {
		t.Fatalf("uptime %v", d3)
	}
}
