package certStore

import (
	"testing"
)

func TestReloadSystemCertificates(t *testing.T) {
	// On Windows this is a no-op, should not panic
	ReloadSystemCertificates()
}

func TestCertPool(t *testing.T) {
	// On Windows this always returns nil
	pool := CertPool()
	if pool != nil {
		t.Error("CertPool() should return nil on Windows")
	}
}
