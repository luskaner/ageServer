package wrapper

import (
	"crypto/x509"
	"testing"
)

func TestWrapperDoesNotPanic(t *testing.T) {
	// These wrappers delegate to cert store; they may succeed or fail depending on OS/permissions,
	// but they should not panic and should be callable to increase coverage.
	_, _ = RemoveUserCerts()
	_ = AddUserCerts([]*x509.Certificate{})
	_ = AddUserCerts(nil)
	// Also test with dummy cert
	cert := &x509.Certificate{Raw: []byte("dummy")}
	_ = AddUserCerts([]*x509.Certificate{cert})
}
