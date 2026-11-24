package common

import (
	"slices"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"API.AgeOfEmpires.com":             "api.ageofempires.com",
		"ed603.playfabapi.com:443":         "ed603.playfabapi.com",
		"athens-live.worldsedge.link.com.": "athens-live.worldsedge.link.com",
		"  aoe-api.reliclink.com  ":        "aoe-api.reliclink.com",
	}
	for in, want := range cases {
		if got := NormalizeHost(in); got != want {
			t.Fatalf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildDomainToGameIDsServesSharedAndExclusiveHosts(t *testing.T) {
	SetUseInternet(false)
	t.Cleanup(func() { SetUseInternet(true) })
	served, unserved := BuildDomainToGameIDs(true)
	if len(unserved) != 0 {
		t.Fatalf("with the API tree on nothing is unserved: %v", unserved)
	}
	if got := served["aoe-api.reliclink.com"]; !slices.Equal(got, []string{"age1", "age2", "age3", "age4"}) {
		t.Fatalf("relic host = %v", got)
	}
	if got := served["cdn.ageofempires.com"]; !slices.Equal(got, []string{"age1", "age2", "age3", "age4", "athens"}) {
		t.Fatalf("cdn host = %v", got)
	}
	if got := served["ed603.playfabapi.com"]; !slices.Equal(got, []string{"age4"}) {
		t.Fatalf("ed603 host = %v", got)
	}
	if got := served["c15f9.playfabapi.com"]; !slices.Equal(got, []string{"athens"}) {
		t.Fatalf("c15f9 host = %v", got)
	}
	if got := served["arthurlive-release1-api.worldsedgelink.com"]; !slices.Equal(got, []string{"age2"}) {
		t.Fatalf("macOS exclusive host = %v", got)
	}
}

func TestBuildDomainToGameIDsMarksApiHostsUnserved(t *testing.T) {
	SetUseInternet(false)
	t.Cleanup(func() { SetUseInternet(true) })
	served, unserved := BuildDomainToGameIDs(false)
	if !unserved["api.ageofempires.com"] {
		t.Fatal("api.ageofempires.com must be unserved when the API tree is off")
	}
	if !unserved["api-dr.ageofempires.com"] {
		t.Fatal("the AoE IV API host must be unserved too")
	}
	if _, ok := served["api.ageofempires.com"]; ok {
		t.Fatal("an unserved host must not be in the served map")
	}
	if got := served["aoe-api.reliclink.com"]; len(got) != 4 {
		t.Fatalf("root hosts must stay served, got %v", got)
	}
}

func TestCertModes(t *testing.T) {
	if GameCertMode("age4") != CertModeModern || GameCertMode("athens") != CertModeModern {
		t.Fatal("age4 and athens are modern")
	}
	for _, legacy := range []string{"age1", "age2", "age3"} {
		if GameCertMode(legacy) != CertModeLegacy {
			t.Fatalf("%s must be legacy", legacy)
		}
		if !SelfSignedCertGame(legacy) {
			t.Fatalf("%s must keep the self-signed leaf", legacy)
		}
	}
	if got := PreferredModes("age2"); !slices.Equal(got, []CertMode{CertModeLegacy, CertModeModern}) {
		t.Fatalf("age2 modes = %v", got)
	}
	if got := PreferredModes("age4"); !slices.Equal(got, []CertMode{CertModeModern, CertModeLegacy}) {
		t.Fatalf("age4 modes = %v", got)
	}
}
