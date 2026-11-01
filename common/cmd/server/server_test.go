package server

import (
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

func noopRun(*pflag.FlagSet) (err error, exitCode int) { return }

func TestSingleFlagSetRegistrations(t *testing.T) {
	values, singleFs := SingleFlagSet("v-test", []string{"cfgdir"}, noopRun)
	if singleFs == nil {
		t.Fatal("SingleFlagSet returned nil")
	}
	for _, name := range []string{
		"config", "announce", "announcePort", "announceMulticast",
		"announceMulticastGroup", "log", "flatLog", "deterministic",
		"games", "logRoot", "generatePlatformUserId", "id", "help", "version",
		"internet",
	} {
		if singleFs.Fs().Lookup(name) == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
	if got := singleFs.Fs().Lookup("announcePort").DefValue; got != itoa(common.AnnouncePort) {
		t.Errorf("announcePort default = %q, want %q", got, itoa(common.AnnouncePort))
	}
	if values.Announce != "true" || values.AnnounceMulticast != "true" {
		t.Errorf("announce defaults = %q/%q, want true/true", values.Announce, values.AnnounceMulticast)
	}
	if !values.CanUseInternet {
		t.Error("CanUseInternet should default to true")
	}
}

func TestSingleFlagSetInternetOffEmitsFlag(t *testing.T) {
	values, singleFs := SingleFlagSet("v-test", nil, noopRun)
	values.CanUseInternet = false
	args := cmd.FlagSetToArgs(singleFs.Fs(), false)
	found := false
	for _, a := range args {
		if a == "--internet=false" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --internet=false in args, got %v", args)
	}
}

func TestSingleFlagSetInternetDefaultOmitted(t *testing.T) {
	_, singleFs := SingleFlagSet("v-test", nil, noopRun)
	args := cmd.FlagSetToArgs(singleFs.Fs(), false)
	for _, a := range args {
		if a == "--internet=false" || a == "--internet" {
			t.Fatalf("internet-enabled default should not be emitted, got %v", args)
		}
	}
}

func TestSingleFlagSetParse(t *testing.T) {
	values, singleFs := SingleFlagSet("v-test", nil, noopRun)
	if err := singleFs.Fs().Parse([]string{
		"--announce=false", "--deterministic", "--id=my-id",
	}); err != nil {
		t.Fatal(err)
	}
	if values.Announce != "false" || !values.Deterministic || values.Id != "my-id" {
		t.Fatalf("values = %+v", values)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
