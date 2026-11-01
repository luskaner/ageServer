package launcher

import (
	"runtime"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// The accessors and the validators must agree, because they answer the same
// question for two different readers: the validator for a run that has already
// been configured, the accessor for a frontend building the control that
// configures it.
//
// They are the same code today. What is being pinned is that they stay the same,
// since the failure would be invisible until a run rejected a value its own form
// had offered, and would then look like a bug in the run.
func TestTheAccessorsAndTheValidatorsAgree(t *testing.T) {
	for _, tc := range []struct {
		name       string
		offered    func() []string
		validate   func(string) int
		unexpected int
	}{
		{
			name:       "serverStart",
			offered:    func() []string { return AutoTrueFalseValues().ToSlice() },
			validate:   func(v string) int { return ValidateServerStartValue(Discard{}, v) },
			unexpected: ErrInvalidServerStart,
		},
		{
			name:       "canTrustCertificate",
			offered:    func() []string { return CanTrustCertificateValues().ToSlice() },
			validate:   func(v string) int { return ValidateCanTrustCertificate(Discard{}, v) },
			unexpected: ErrInvalidCanTrustCertificate,
		},
		{
			name:       "canBroadcastBattleServer",
			offered:    func() []string { return CanBroadcastBattleServerValues().ToSlice() },
			validate:   func(v string) int { return ValidateCanBroadcastBattleServer(Discard{}, v) },
			unexpected: ErrInvalidCanBroadcastBattleServer,
		},
		{
			name:     "serverStop",
			offered:  func() []string { return ServerStopValues(true).ToSlice() },
			validate: func(v string) int { return ValidateServerStopValue(Discard{}, v, true) },
			// The run is already administrator off windows, which is the only
			// case where the set is smaller.
			unexpected: ErrInvalidServerStop,
		},
		{
			name:    "requiredTrueFalse",
			offered: func() []string { return RequiredTrueFalseValues().ToSlice() },
			validate: func(v string) int {
				return ValidateRequiredTrueFalse(Discard{}, v, "Server.BattleServerManager.Run", RequiredTrueFalseValues())
			},
			unexpected: ErrInvalidServerBattleServerManagerRun,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range tc.offered() {
				if ec := tc.validate(v); ec != common.ErrSuccess {
					t.Errorf("%q is offered by the accessor but the validator rejects it with %d", v, ec)
				}
			}
			// A value none of them offer, so an accessor that had quietly stopped
			// filtering would still fail here.
			if ec := tc.validate("definitely-not-a-value"); ec != tc.unexpected {
				t.Errorf("the validator accepted a value the accessor does not offer (exit code %d)", ec)
			}
		})
	}
}

// The per-user trust store does not exist on linux, so a value the run cannot act
// on must not be offered there. This is the one answer that depends on the
// platform, which is why it is worth a test of its own.
func TestCanTrustCertificateValuesDropUserOnLinux(t *testing.T) {
	offered := CanTrustCertificateValues().Contains("user")
	if linux := runtime.GOOS == "linux"; offered == linux {
		t.Errorf("on %s the accessor offers the user store: %v, want %v", runtime.GOOS, offered, !linux)
	}
}

// The accessors hand out sets of their own. A frontend that emptied one would
// otherwise leave every later run rejecting everything.
func TestTheAccessorsHandOutTheirOwnSets(t *testing.T) {
	first := AutoTrueFalseValues()
	if !first.Contains(ModeTrue) {
		t.Fatalf("the first accessor handed out a set without %q in it", ModeTrue)
	}
	first.Clear()
	if !AutoTrueFalseValues().Contains(ModeTrue) {
		t.Error("emptying one accessor's set emptied the next caller's")
	}
}
