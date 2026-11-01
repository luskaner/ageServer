package common

import (
	"net"
	"net/netip"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	commonGame "github.com/luskaner/ageServer/common/game"
)

func TestNetIPToNetIPAddrIPv4(t *testing.T) {
	ip := net.ParseIP("192.168.1.10")
	got := NetIPToNetIPAddr(ip)
	want := netip.MustParseAddr("192.168.1.10")
	if got != want {
		t.Fatalf("NetIPToNetIPAddr = %v, want %v", got, want)
	}
}

func TestNetIPAddrToNetIPRoundTrip(t *testing.T) {
	for _, s := range []string{"0.0.0.0", "127.0.0.1", "192.168.0.25"} {
		addr := netip.MustParseAddr(s)
		back := NetIPAddrToNetIP(addr)
		if !back.Equal(net.ParseIP(s)) {
			t.Fatalf("round trip of %s = %v", s, back)
		}
		if again := NetIPToNetIPAddr(back); again != addr {
			t.Fatalf("double round trip of %s = %v", s, again)
		}
	}
}

func TestStringSliceToNetIPSlice(t *testing.T) {
	got := StringSliceToNetIPSlice([]string{"1.2.3.4", "not-an-ip", "5.6.7.8"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (invalid entries dropped)", len(got))
	}
	if !got[0].Equal(net.ParseIP("1.2.3.4")) || !got[1].Equal(net.ParseIP("5.6.7.8")) {
		t.Fatalf("got %v", got)
	}
}

func TestNetIPSliceToNetIPSet(t *testing.T) {
	set := NetIPSliceToNetIPSet([]net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("1.2.3.4"), net.ParseIP("5.6.7.8")})
	want := mapset.NewThreadUnsafeSet(
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("5.6.7.8"),
	)
	if !set.Equal(want) {
		t.Fatalf("set = %v, want %v", set, want)
	}
}

func TestCertDomainsAndSelfSigned(t *testing.T) {
	domains := CertDomains()
	if len(domains) == 0 {
		t.Fatal("CertDomains returned nothing")
	}
	for _, d := range domains {
		if d == "" {
			t.Fatal("empty domain in CertDomains")
		}
	}
	if SelfSignedCertGame(commonGame.AoE4) || SelfSignedCertGame(commonGame.AoM) {
		t.Fatal("AoE4 and AoM must not use self-signed certs")
	}
	for _, g := range []string{commonGame.AoE1, commonGame.AoE2, commonGame.AoE3} {
		if !SelfSignedCertGame(g) {
			t.Errorf("%s must use self-signed certs", g)
		}
	}
}
