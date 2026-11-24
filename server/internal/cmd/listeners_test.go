package cmd

import (
	"net/netip"
	"slices"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
)

func addrSet(addrs ...string) mapset.Set[netip.Addr] {
	set := mapset.NewThreadUnsafeSet[netip.Addr]()
	for _, addr := range addrs {
		set.Add(netip.MustParseAddr(addr))
	}
	return set
}

func TestValidateListenerSets(t *testing.T) {
	modern := addrSet("192.168.1.10")
	legacy := addrSet("192.168.1.17")
	if err := validateListenerSets(modern, legacy); err != nil {
		t.Fatalf("two distinct addresses are valid: %v", err)
	}
	if err := validateListenerSets(addrSet("0.0.0.0"), addrSet()); err != nil {
		t.Fatalf("0.0.0.0 alone is valid: %v", err)
	}
	if err := validateListenerSets(addrSet("0.0.0.0"), addrSet("192.168.1.17")); err == nil {
		t.Fatal("0.0.0.0 plus a legacy address must fail")
	}
	if err := validateListenerSets(addrSet("0.0.0.0", "192.168.1.10"), addrSet()); err == nil {
		t.Fatal("0.0.0.0 plus another address in the same list must fail")
	}
	if err := validateListenerSets(addrSet("192.168.1.10"), addrSet("192.168.1.10")); err == nil {
		t.Fatal("intersection must fail")
	}
}

func TestDefaultListenerHosts(t *testing.T) {
	placeholder := []string{"0.0.0.0"}
	cases := []struct {
		name       string
		modern     []string
		legacy     []string
		games      []string
		wantModern []string
		wantLegacy []string
		wantErr    bool
	}{
		{
			name: "placeholder follows a legacy game", modern: placeholder, legacy: placeholder, games: []string{"age2"},
			wantLegacy: placeholder,
		},
		{
			name: "a modern-only placeholder follows a legacy game", modern: placeholder, games: []string{"age2"},
			wantLegacy: placeholder,
		},
		{
			name: "placeholder follows a modern game", modern: placeholder, legacy: placeholder, games: []string{"age4"},
			wantModern: placeholder,
		},
		{
			name: "both classes on one address is an error", modern: placeholder, legacy: placeholder, games: []string{"age2", "age4"},
			wantErr: true,
		},
		{
			name: "a modern-only placeholder cannot serve both classes", modern: placeholder, games: []string{"age2", "age4"},
			wantErr: true,
		},
		{
			name: "placeholder with no games keeps modern", modern: placeholder, legacy: placeholder, games: nil,
			wantModern: placeholder,
		},
		{
			name: "a configured address is left alone", modern: []string{"192.0.2.1"}, games: []string{"age2"},
			wantModern: []string{"192.0.2.1"},
		},
		{
			name: "a configured legacy list is left alone", modern: placeholder, legacy: []string{"192.0.2.2"}, games: []string{"age4"},
			wantModern: placeholder, wantLegacy: []string{"192.0.2.2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotModern, gotLegacy, err := defaultListenerHosts(tc.modern, tc.legacy, tc.games)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if !slices.Equal(gotModern, tc.wantModern) {
				t.Fatalf("modern = %#v, want %#v", gotModern, tc.wantModern)
			}
			if !slices.Equal(gotLegacy, tc.wantLegacy) {
				t.Fatalf("legacy = %#v, want %#v", gotLegacy, tc.wantLegacy)
			}
		})
	}
}

func TestPlaceholderHosts(t *testing.T) {
	if !placeholderHosts([]string{"0.0.0.0"}, nil) {
		t.Fatal("a single 0.0.0.0 list is the placeholder")
	}
	if !placeholderHosts([]string{"0.0.0.0"}, []string{"0.0.0.0"}) {
		t.Fatal("0.0.0.0 in both lists is the placeholder")
	}
	if placeholderHosts([]string{"192.0.2.1"}, nil) {
		t.Fatal("a concrete address is not the placeholder")
	}
	if placeholderHosts([]string{"0.0.0.0"}, []string{"192.0.2.2"}) {
		t.Fatal("a configured legacy list is not the placeholder")
	}
	if placeholderHosts(nil, nil) {
		t.Fatal("no listener is not served by the derivation")
	}
}

func TestBuildListenerReport(t *testing.T) {
	specs := []listenerSpec{
		{Addr: netip.MustParseAddr("192.168.1.10"), Mode: common.CertModeModern},
		{Addr: netip.MustParseAddr("192.168.1.17"), Mode: common.CertModeLegacy},
	}
	got := buildListenerReport(specs, []string{"age2", "age4"})
	want := []string{
		"Listener report:",
		"  192.168.1.10:443  certificates: modern",
		"  192.168.1.17:443  certificates: legacy",
		"",
		"Game age2 (class legacy):",
		"  WARN  192.168.1.10:443 (modern) — AoE I-III: DE may reject it (see M0)",
		"  OK    192.168.1.17:443 (legacy)",
		"",
		"Game age4 (class modern):",
		"  OK    192.168.1.10:443 (modern)",
		"  WARN  192.168.1.17:443 (legacy) — AoE IV / AoM may reject it",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("report =\n%s\nwant\n%s", join(got), join(want))
	}
}

func join(lines []string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
