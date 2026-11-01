package internal

import "testing"

func TestErrorConstants(t *testing.T) {
	// Verify error constants have unique, non-zero values
	seen := make(map[int]bool)
	constants := []struct {
		name string
		val  int
	}{
		{"ErrLocalCertRemove", ErrLocalCertRemove},
		{"ErrIpMapRemove", ErrIpMapRemove},
		{"ErrIpMapRemoveRevert", ErrIpMapRemoveRevert},
		{"ErrLocalCertAdd", ErrLocalCertAdd},
		{"ErrLocalCertAddParse", ErrLocalCertAddParse},
		{"ErrIpMapAdd", ErrIpMapAdd},
		{"ErrIpMapAddRevert", ErrIpMapAddRevert},
		{"ErrFlushCache", ErrFlushCache},
		{"ErrFlushCacheDNS", ErrFlushCacheDNS},
		{"ErrFlushCacheCerts", ErrFlushCacheCerts},
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

func TestSetUpStateInitialized(t *testing.T) {
	if SetUp != nil {
		t.Fatal("SetUp should start as nil")
	}
	f := new(true)
	SetUp = f
	if SetUp == nil || *SetUp != true {
		t.Fatal("SetUp should be settable to true")
	}
	SetUp = nil
}
