package internal

import "testing"

func TestErrorConstants(t *testing.T) {
	seen := make(map[int]bool)
	constants := []struct {
		name string
		val  int
	}{
		{"ErrListen", ErrListen},
		{"ErrDecode", ErrDecode},
		{"ErrNonExistingAction", ErrNonExistingAction},
		{"ErrConnectionClosing", ErrConnectionClosing},
		{"ErrCertAlreadyAdded", ErrCertAlreadyAdded},
		{"ErrIpsAlreadyMapped", ErrIpsAlreadyMapped},
		{"ErrCertInvalid", ErrCertInvalid},
		{"ErrFlushCache", ErrFlushCache},
		{"ErrGameNotSupported", ErrGameNotSupported},
	}
	for _, c := range constants {
		if c.val == 0 {
			t.Errorf("%s should not be 0", c.name)
		}
		if seen[c.val] {
			t.Errorf("%s has duplicate value %d", c.name, c.val)
		}
		seen[c.val] = true
	}
}
