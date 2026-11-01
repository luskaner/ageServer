package data

import (
	"testing"
	"time"
)

// Regression: Update used a value receiver, so ct.Time = time.Now() mutated a
// copy — the timestamp was never actually updated.
func TestCustomTimeUpdate(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	ct := CustomTime{Time: old}
	ct.Update()
	if !ct.Time.After(old) {
		t.Fatalf("Update did not advance time: still %v", ct.Time)
	}
}

func TestBaseValueUpdateLastUpdated(t *testing.T) {
	bv := NewBaseValue("Public", "hello")
	before := bv.LastUpdated.Time
	time.Sleep(10 * time.Millisecond)
	bv.UpdateLastUpdated()
	if !bv.LastUpdated.Time.After(before) {
		t.Fatal("UpdateLastUpdated did not advance the timestamp (value receiver no-op)")
	}
}

func TestBaseValueUpdateModifiesValueAndTimestamp(t *testing.T) {
	bv := NewBaseValue[string]("Private", "old")
	bv.Update(func(v *string) { *v = "new" })
	if *bv.Value != "new" {
		t.Fatalf("value = %q, want %q", *bv.Value, "new")
	}
}
