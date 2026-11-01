//go:build !linux

package cert

import "testing"

func TestFlushCerts_NonLinux(t *testing.T) {
	res := FlushCerts()
	if res == nil {
		t.Fatal("nil")
	}
	if res.Err != nil {
		t.Errorf("unexpected result: %+v", res)
	}
	// Should be success
	if !res.Success() {
		t.Error("expected success")
	}
}
