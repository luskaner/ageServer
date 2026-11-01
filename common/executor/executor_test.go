package executor

import "testing"

func TestIsAdmin(t *testing.T) {
	// IsAdmin makes a real Windows syscall
	// Just verify it doesn't panic and returns a bool
	_ = IsAdmin()
}

func TestIsAdminResult(t *testing.T) {
	admin := IsAdmin()
	t.Logf("IsAdmin() = %v", admin)
}
