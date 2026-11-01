package launcher

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// The validators now report through a Reporter instead of printing, so these
// tests hand them one that throws everything away. What is under test is the
// exit code, which is what a frontend needs to decide what to do.

func TestValidationCanTrustCertificateInvalid(t *testing.T) {
	exitCode := ValidateCanTrustCertificate(Discard{}, "invalid-value")
	if exitCode != ErrInvalidCanTrustCertificate {
		t.Errorf("expected exit code %d, got %d", ErrInvalidCanTrustCertificate, exitCode)
	}
}

func TestValidationCanTrustCertificateValid(t *testing.T) {
	for _, val := range []string{"auto", "false", "local"} {
		exitCode := ValidateCanTrustCertificate(Discard{}, val)
		if exitCode != common.ErrSuccess {
			t.Errorf("for canTrustCertificate=%s, expected success, got %d", val, exitCode)
		}
	}
	if runtime.GOOS == "darwin" {
		if exitCode := ValidateCanTrustCertificate(Discard{}, "user"); exitCode != common.ErrSuccess {
			t.Errorf("expected success for 'user' on darwin, got %d", exitCode)
		}
	}
	if exitCode := ValidateCanTrustCertificate(Discard{}, "local"); exitCode != common.ErrSuccess {
		t.Errorf("expected success for 'local', got %d", exitCode)
	}
}

func TestValidationServerStartInvalid(t *testing.T) {
	exitCode := ValidateServerStartValue(Discard{}, "invalid")
	if exitCode != ErrInvalidServerStart {
		t.Errorf("expected exit code %d, got %d", ErrInvalidServerStart, exitCode)
	}
}

func TestValidationServerStopInvalid(t *testing.T) {
	for _, stop := range []string{"auto", "true", "false"} {
		if exitCode := ValidateServerStopValue(Discard{}, stop, false); exitCode != common.ErrSuccess {
			t.Errorf("for serverStop=%s (non-admin), expected success, got %d", stop, exitCode)
		}
	}
	exitCode := ValidateServerStopValue(Discard{}, "invalid-value", true)
	if exitCode != ErrInvalidServerStop {
		t.Errorf("expected exit code %d, got %d", ErrInvalidServerStop, exitCode)
	}
}

func TestValidationCanBroadcastBattleServer(t *testing.T) {
	if ec := ValidateCanBroadcastBattleServer(Discard{}, "auto"); ec != common.ErrSuccess {
		t.Errorf("expected success for auto, got %d", ec)
	}
	if ec := ValidateCanBroadcastBattleServer(Discard{}, "false"); ec != common.ErrSuccess {
		t.Errorf("expected success for false, got %d", ec)
	}
	if ec := ValidateCanBroadcastBattleServer(Discard{}, "true"); ec != ErrInvalidCanBroadcastBattleServer {
		t.Errorf("expected invalid for true, got %d", ec)
	}
	if ec := ValidateCanBroadcastBattleServer(Discard{}, "bad"); ec != ErrInvalidCanBroadcastBattleServer {
		t.Errorf("expected invalid for bad, got %d", ec)
	}
}

func TestValidationRequiredTrueFalse(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		field    string
		wantCode int
	}{
		{"valid true", "true", "Server.BattleServerManager.Run", common.ErrSuccess},
		{"valid required", "required", "Client.Isolation.Metadata", common.ErrSuccess},
		{"invalid", "bad", "Server.BattleServerManager.Run", ErrInvalidServerBattleServerManagerRun},
		{"invalid metadata", "bad", "Client.Isolation.Metadata", ErrInvalidIsolateMetadata},
		{"invalid profiles", "bad", "Client.Isolation.Profiles", ErrInvalidIsolateProfiles},
	}
	for _, tt := range tests {
		ec := ValidateRequiredTrueFalse(Discard{}, tt.value, tt.field, RequiredTrueFalseValues())
		if ec != tt.wantCode {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.wantCode, ec)
		}
	}
}

// A rejected value has to say which values would have worked. A frontend showing
// the same message the console does is the whole point of moving these here, and
// a message without the options is a dead end.
func TestValidationSaysWhichValuesAreAccepted(t *testing.T) {
	r := &recordingReporter{}
	ValidateServerStopValue(r, "nope", true)
	if len(r.failed) != 1 {
		t.Fatalf("got %d failures, want 1: %q", len(r.failed), r.failed)
	}
	for _, want := range []string{"serverStop", "auto", "true"} {
		if !strings.Contains(r.failed[0], want) {
			t.Errorf("%q does not mention %q", r.failed[0], want)
		}
	}
}

type recordingReporter struct {
	Discard
	failed []string
}

func (r *recordingReporter) Fail(format string, a ...any) {
	r.failed = append(r.failed, fmt.Sprintf(format, a...))
}
