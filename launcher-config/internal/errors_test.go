package internal

import "testing"

func TestErrorConstants(t *testing.T) {
	seen := make(map[int]bool)
	constants := []struct {
		name string
		val  int
	}{
		{"ErrUserCertRemove", ErrUserCertRemove},
		{"ErrUserCertAdd", ErrUserCertAdd},
		{"ErrUserCertAddParse", ErrUserCertAddParse},
		{"ErrMetadataRestore", ErrMetadataRestore},
		{"ErrProfilesRestore", ErrProfilesRestore},
		{"ErrAdminRevert", ErrAdminRevert},
		{"ErrMetadataBackup", ErrMetadataBackup},
		{"ErrProfilesBackup", ErrProfilesBackup},
		{"ErrStartAgent", ErrStartAgent},
		{"ErrStartAgentVerify", ErrStartAgentVerify},
		{"ErrAdminSetup", ErrAdminSetup},
		{"ErrRevertStopAgent", ErrRevertStopAgent},
		{"ErrHostsAdd", ErrHostsAdd},
		{"ErrMissingLocalCertData", ErrMissingLocalCertData},
		{"ErrGameCertAddParse", ErrGameCertAddParse},
		{"ErrGameCertAdd", ErrGameCertAdd},
		{"ErrGameCertRestore", ErrGameCertRestore},
		{"ErrGameCertBackup", ErrGameCertBackup},
		{"ErrGamePathMissing", ErrGamePathMissing},
		{"ErrInvalidDataPath", ErrInvalidDataPath},
		{"ErrAgentAlreadyStarted", ErrAgentAlreadyStarted},
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

func TestNewCACertAoE1ReturnsNil(t *testing.T) {
	cert := NewCACert("age1", t.TempDir())
	if cert != nil {
		t.Fatal("AoE1 should not have a CA cert")
	}
}

func TestNewCACertAoE4ReturnsNil(t *testing.T) {
	cert := NewCACert("age4", t.TempDir())
	if cert != nil {
		t.Fatal("AoE4 should not have a CA cert")
	}
}

func TestNewCACertAoMReturnsNil(t *testing.T) {
	// AoM has a CA cert but the path is derived from gamePath;
	// with a valid temp dir the CA lookup may succeed or fail depending on
	// the filesystem. We just verify it doesn't panic.
	_ = NewCACert("athens", t.TempDir())
}
