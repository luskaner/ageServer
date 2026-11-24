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

func TestBuildListenerReport(t *testing.T) {
	specs := []listenerSpec{
		{Addr: netip.MustParseAddr("192.168.1.10"), Mode: common.CertModeModern},
		{Addr: netip.MustParseAddr("192.168.1.17"), Mode: common.CertModeLegacy},
	}
	got := buildListenerReport(specs, []string{"age2", "age4"}, "")
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
	filtered := buildListenerReport(specs, []string{"age4"}, common.CertModeLegacy)
	if filtered[0] != "Listener report (listening only: legacy):" {
		t.Fatalf("filter title = %q", filtered[0])
	}
}

func join(lines []string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
