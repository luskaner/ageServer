package executor

import (
	"encoding/base64"
	"net"
	"slices"
	"testing"

	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executor/exec"
)

func TestNewConfigSetupOptions(t *testing.T) {
	opts := NewConfigSetupOptions()
	if opts == nil {
		t.Fatal("NewConfigSetupOptions() returned nil")
	}
	if opts.SetupValues == nil {
		t.Fatal("SetupValues is nil")
	}
	if opts.flags == nil {
		t.Fatal("flags is nil")
	}
}

func TestConfigRevertFlagOptionsAllNil(t *testing.T) {
	opts := NewConfigSetupOptions()
	revert := opts.ConfigRevertFlagOptions()
	if revert == nil {
		t.Fatal("ConfigRevertFlagOptions() returned nil")
	}
	if revert.IPs {
		t.Error("IPs should be false when MapIp is nil")
	}
	if revert.Certs {
		t.Error("Certs should be false when AddLocalCertData is nil")
	}
	if revert.RemoveUserCert {
		t.Error("RemoveUserCert should be false when AddUserCertData is nil")
	}
	if revert.RestoreCAStoreCert {
		t.Error("RestoreCAStoreCert should be false when AddCACertData is nil")
	}
}

func TestConfigRevertFlagOptionsWithValues(t *testing.T) {
	opts := NewConfigSetupOptions()
	opts.MapIp = net.ParseIP("127.0.0.1")
	opts.AddLocalCertData = []byte("cert")
	opts.AddUserCertData = []byte("user")
	opts.AddCACertData = []byte("ca")
	opts.GameId = "age2"
	opts.LogRoot = "/tmp/logs"
	opts.CertFilePath = "/tmp/cert.pem"
	opts.SigKeyFilePath = "/tmp/sigkey.der"
	opts.HostFilePath = "/tmp/hosts"
	opts.DataPath = "/tmp/data"
	opts.GamePath = "/tmp/game"
	opts.Metadata = true
	opts.Profiles = true

	revert := opts.ConfigRevertFlagOptions()
	if !revert.IPs {
		t.Error("IPs should be true when MapIp is set")
	}
	if !revert.Certs {
		t.Error("Certs should be true when AddLocalCertData is set")
	}
	if !revert.RemoveUserCert {
		t.Error("RemoveUserCert should be true when AddUserCertData is set")
	}
	if !revert.RestoreCAStoreCert {
		t.Error("RestoreCAStoreCert should be true when AddCACertData is set")
	}
	if revert.GameId != "age2" {
		t.Errorf("GameId = %q, want %q", revert.GameId, "age2")
	}
	if revert.LogRoot != "/tmp/logs" {
		t.Errorf("LogRoot = %q, want %q", revert.LogRoot, "/tmp/logs")
	}
	if revert.CertFilePath != "/tmp/cert.pem" {
		t.Errorf("CertFilePath = %q, want %q", revert.CertFilePath, "/tmp/cert.pem")
	}
	if revert.SigKeyFilePath != "/tmp/sigkey.der" {
		t.Errorf("SigKeyFilePath = %q, want %q", revert.SigKeyFilePath, "/tmp/sigkey.der")
	}
	if revert.HostFilePath != "/tmp/hosts" {
		t.Errorf("HostFilePath = %q, want %q", revert.HostFilePath, "/tmp/hosts")
	}
	if revert.DataPath != "/tmp/data" {
		t.Errorf("DataPath = %q, want %q", revert.DataPath, "/tmp/data")
	}
	if revert.GamePath != "/tmp/game" {
		t.Errorf("GamePath = %q, want %q", revert.GamePath, "/tmp/game")
	}
	if !revert.Metadata {
		t.Error("Metadata should be true")
	}
	if !revert.Profiles {
		t.Error("Profiles should be true")
	}
}

// The revert arguments are built from the same flag set the setup runs with, so
// a path the setup is told to write has to be a path the revert is told to
// remove. Without this the temporary key file would be written and then left
// behind when the game exits.
func TestConfigSetupEmitsTheSigningKeyFlags(t *testing.T) {
	opts := NewConfigSetupOptions()
	opts.GameId = "age2"
	opts.SigKeyFilePath = "/tmp/sigkey.der"
	keyData := []byte{0x30, 0x01}
	opts.AddSigKeyData = keyData

	args := cmd.FlagSetToArgs(opts.flags, true)
	for _, want := range []string{
		"--sigKeyFilePath=/tmp/sigkey.der",
		"--sigKeyData=" + base64.StdEncoding.EncodeToString(keyData),
	} {
		if !slices.Contains(args, want) {
			t.Errorf("setup args %v do not contain %q", args, want)
		}
	}

	revertFlags := opts.ConfigRevertFlagOptions().Flags()
	if !slices.Contains(revertFlags, "--sigKeyFilePath=/tmp/sigkey.der") {
		t.Errorf("revert flags %v do not contain the key file path, so it would never be removed", revertFlags)
	}
}

func TestNewConfigFlushCacheOptionsAllDisabled(t *testing.T) {
	opts := NewConfigFlushCacheOptions(false, "false", true, true)
	if opts != nil {
		t.Error("expected nil when both ips and certs disabled")
	}
}

func TestNewConfigFlushCacheOptionsIPsOnly(t *testing.T) {
	opts := NewConfigFlushCacheOptions(true, "false", false, false)
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if !opts.IPs {
		t.Error("IPs should be true")
	}
}

func TestRunRevertInvalidFlags(t *testing.T) {
	result := RunRevert([]string{"--invalid-flag-value"}, false, nil, func(opts *exec.Options) {})
	if result.Err == nil {
		t.Error("expected error for invalid flags")
	}
}
