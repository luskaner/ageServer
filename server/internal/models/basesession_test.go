package models

import (
	"testing"
	"time"
)

// Regression: nextExpiration could return a zero or negative duration when a
// session's expiry fell exactly at `now`, causing time.NewTicker to panic in
// the sweeper goroutine (killing the process).
func TestNextExpirationAlwaysPositive(t *testing.T) {
	sessions := NewBaseSessions[string, string](5 * time.Minute)

	// Add a session that expires exactly at now (boundary case).
	now := time.Now().UTC()
	sessions.store.Store("boundary", &BaseSession[string, string]{
		id:     "boundary",
		expiry: now,
	}, nil)

	_, duration := sessions.nextExpiration()
	if duration <= 0 {
		t.Fatalf("nextExpiration = %v, want > 0", duration)
	}
}

func TestNextExpirationWithFutureSession(t *testing.T) {
	sessions := NewBaseSessions[string, string](5 * time.Minute)
	future := time.Now().UTC().Add(10 * time.Minute)
	sessions.store.Store("future", &BaseSession[string, string]{
		id:     "future",
		expiry: future,
	}, nil)

	alreadyExpired, duration := sessions.nextExpiration()
	if len(alreadyExpired) != 0 {
		t.Fatalf("unexpected expired entries: %v", alreadyExpired)
	}
	if duration <= 0 {
		t.Fatalf("duration = %v, want positive", duration)
	}
}

func TestNextExpirationNoSessionsUsesDefault(t *testing.T) {
	const defaultExpiry = 5 * time.Minute
	sessions := NewBaseSessions[string, string](defaultExpiry)
	_, _ = sessions.nextExpiration()
}
