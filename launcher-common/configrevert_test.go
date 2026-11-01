package launcher_common

import (
	"testing"

	"github.com/luskaner/ageServer/launcher-common/cmd/config"
)

func TestNewConfigRevertFlagOptions(t *testing.T) {
	opts := NewConfigRevertFlagOptions()
	if opts == nil {
		t.Fatal("NewConfigRevertFlagOptions returned nil")
	}
	if opts.RevertValues == nil {
		t.Fatal("RevertValues should be initialized")
	}
}

func TestConfigRevertFlagOptions_FlagsDefaults(t *testing.T) {
	opts := NewConfigRevertFlagOptions()
	flags := opts.Flags()
	// Default: no flags set, should return empty or only logRoot/game
	for _, f := range flags {
		if f == "--logRoot" || f == "--game" {
			continue
		}
		t.Errorf("unexpected default flag: %q", f)
	}
}

func TestConfigRevertFlagOptions_FlagsRemoveAllClearsOthers(t *testing.T) {
	opts := NewConfigRevertFlagOptions()
	opts.RemoveAll = true
	opts.IPs = true
	opts.Certs = true
	opts.Metadata = true
	opts.Profiles = true
	opts.RemoveUserCert = true
	opts.RestoreCAStoreCert = true

	flags := opts.Flags()
	// When RemoveAll is true, individual flags should be cleared
	for _, f := range flags {
		if f == "--ip" || f == "--localCert" || f == "--metadata" || f == "--profiles" || f == "--userCert" || f == "--caStoreCert" {
			t.Errorf("RemoveAll should clear individual flag %q, but it was emitted", f)
		}
	}
}

func TestRevertRequiresAdminElevationValues(t *testing.T) {
	tests := []struct {
		name   string
		values config.RevertValues
		want   bool
	}{
		{"nothing set", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{}},
			CommonBaseValues: &config.CommonBaseValues{},
		}, false},
		{"certs without path", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{Certs: true}},
			CommonBaseValues: &config.CommonBaseValues{},
		}, true},
		{"certs with path", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{Certs: true}},
			CommonBaseValues: &config.CommonBaseValues{CertFilePath: "/cert"},
		}, false},
		{"ips without path", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{IPs: true}},
			CommonBaseValues: &config.CommonBaseValues{},
		}, true},
		{"ips with path", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{IPs: true}},
			CommonBaseValues: &config.CommonBaseValues{HostFilePath: "/hosts"},
		}, false},
		{"both without paths", config.RevertValues{
			RevertBaseValues: &config.RevertBaseValues{RevertMinimalValues: &config.RevertMinimalValues{Certs: true, IPs: true}},
			CommonBaseValues: &config.CommonBaseValues{},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RevertRequiresAdminElevationValues(&tt.values)
			if got != tt.want {
				t.Errorf("RevertRequiresAdminElevationValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigRevertFlagOptions_RestoreAfterFlags(t *testing.T) {
	opts := NewConfigRevertFlagOptions()
	opts.RemoveAll = true
	opts.IPs = true
	opts.Certs = true
	_ = opts.Flags()
	// After Flags(), original values must be restored (non-mutating fix)
	if !opts.IPs || !opts.Certs {
		t.Error("Flags() should restore original values after clearing, not permanently mutate")
	}
}

func TestConfigRevertFlagOptions_WithGameAndLogRoot(t *testing.T) {
	opts := NewConfigRevertFlagOptions()
	opts.GameId = "age2"
	opts.LogRoot = "/tmp/logs"
	opts.IPs = true
	flags := opts.Flags()
	foundGame, foundLog := false, false
	for i, f := range flags {
		if f == "--game=age2" || f == "--game" && i+1 < len(flags) && flags[i+1] == "age2" {
			foundGame = true
		}
		if f == "--logRoot=/tmp/logs" || f == "--logRoot" && i+1 < len(flags) && flags[i+1] == "/tmp/logs" {
			foundLog = true
		}
		// Also check joined form via FlagSetToArgs uses --name=value
		if f == "--game=age2" {
			foundGame = true
		}
		if f == "--logRoot=/tmp/logs" {
			foundLog = true
		}
	}
	// FlagSetToArgs emits --game=age2 etc; ensure at least one is present
	hasGame := false
	for _, f := range flags {
		if f == "--game=age2" {
			hasGame = true
		}
	}
	if !hasGame {
		t.Errorf("expected --game flag, got %v", flags)
	}
	if !foundLog && len(flags) > 0 {
		// LogRoot may be emitted as "--logRoot=/tmp/logs"
		hasLog := false
		for _, f := range flags {
			if f == "--logRoot=/tmp/logs" {
				hasLog = true
			}
		}
		if !hasLog {
			t.Errorf("expected --logRoot flag, got %v", flags)
		}
	}
	_ = foundGame
	_ = foundLog
}

func TestAllRevertFlagsContainsAll(t *testing.T) {
	flags := allRevertFlags("age2", "/logs")
	found := false
	for _, f := range flags {
		if f == "--all" {
			found = true
		}
	}
	if !found {
		t.Fatalf("allRevertFlags should contain --all, got %v", flags)
	}
	// Should also contain game and logRoot if provided
	hasGame := false
	for _, f := range flags {
		if f == "--game=age2" {
			hasGame = true
		}
	}
	if !hasGame {
		t.Errorf("allRevertFlags missing game, got %v", flags)
	}
}
