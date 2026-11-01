package ipc

import "testing"

func TestSetupServerWindowsSuccess(t *testing.T) {
	l, err := SetupServer()
	if err != nil {
		t.Fatalf("SetupServer failed: %v", err)
	}
	if l == nil {
		t.Fatal("listener is nil")
	}
	// Ensure we can close and revert
	_ = l.Close()
	RevertServer()
	// Calling RevertServer should not panic (it's no-op)
	RevertServer()
}

func TestRevertServerNoOp(t *testing.T) {
	// Should not panic
	RevertServer()
}
