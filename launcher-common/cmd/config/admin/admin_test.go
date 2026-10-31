package admin

import (
	"testing"
)

func TestSetupFlagSet(t *testing.T) {
	values, flags := SetupFlagSet()
	if values == nil || flags == nil {
		t.Fatal("SetupFlagSet returned nil")
	}
	for _, name := range []string{"ip", "localCert", "game", "logRoot", "internet"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestSetupFlagSetParsesFlags(t *testing.T) {
	values, flags := SetupFlagSet()
	err := flags.Parse([]string{"--ip", "10.0.0.1", "--game", "aoe2de", "--internet=false"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.MapIp.String() != "10.0.0.1" {
		t.Errorf("MapIp = %v, want 10.0.0.1", values.MapIp)
	}
	if values.GameId != "aoe2de" {
		t.Errorf("GameId = %q, want aoe2de", values.GameId)
	}
	if values.CanUseInternet {
		t.Error("CanUseInternet should be false when flag is false")
	}
}

func TestRevertFlagSet(t *testing.T) {
	values, flags := RevertFlagSet()
	if values == nil || flags == nil {
		t.Fatal("RevertFlagSet returned nil")
	}
	for _, name := range []string{"ip", "localCert", "all", "logRoot"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
	err := flags.Parse([]string{"--ip", "--all"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !values.IPs || !values.RemoveAll {
		t.Error("IPs and RemoveAll should be true")
	}
}
