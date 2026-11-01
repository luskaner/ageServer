package common

import (
	"net"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
)

// White-box tests over the DNS cache maps (pure, no network).
func TestResolveCacheRoundTrip(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	CacheMapping("cached.test", "203.0.113.7")

	if cached, hosts := cachedHostToIps("cached.test"); !cached || hosts == nil || !hosts.Contains("203.0.113.7") {
		t.Fatalf("cachedHostToIps = %v, %v", cached, hosts)
	}
	if cached, hosts := cachedIpToHosts("203.0.113.7"); !cached || hosts == nil || !hosts.Contains("cached.test") {
		t.Fatalf("cachedIpToHosts = %v, %v", cached, hosts)
	}

	ClearDNSCache()
	if cached, _ := cachedHostToIps("cached.test"); cached {
		t.Fatal("ClearDNSCache did not clear host cache")
	}
	if cached, _ := cachedIpToHosts("203.0.113.7"); cached {
		t.Fatal("ClearDNSCache did not clear ip cache")
	}
}

func TestMatchesWithIPLiterals(t *testing.T) {
	if !Matches("1.2.3.4", "1.2.3.4") {
		t.Fatal("identical IP literals must match")
	}
	if Matches("1.2.3.4", "5.6.7.8") {
		t.Fatal("different IP literals must not match")
	}
}

func TestHostOrIpToIpsLiteral(t *testing.T) {
	got := HostOrIpToIps("192.168.5.5")
	if len(got) != 1 || got[0] != "192.168.5.5" {
		t.Fatalf("got %v", got)
	}
	// IPv6 is out of scope for this resolver: no IPv4 mapping -> empty.
	if got := HostOrIpToIps("::1"); len(got) != 0 {
		t.Fatalf("IPv6 literal must yield empty set, got %v", got)
	}
}

func TestHostOrIpToIpsUnspecifiedExpandsToLocal(t *testing.T) {
	got := HostOrIpToIps("0.0.0.0")
	if len(got) == 0 {
		t.Skip("no local IPv4 interfaces available")
	}
	for _, ip := range got {
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			t.Fatalf("non-IPv4 entry %q", ip)
		}
	}
}

func TestCacheMappingAccumulatesHostsPerIp(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	CacheMapping("a.test", "198.51.100.9")
	CacheMapping("b.test", "198.51.100.9")

	hosts := IpToHosts("198.51.100.9")
	want := mapset.NewThreadUnsafeSet[string]("a.test", "b.test")
	if hosts == nil || !hosts.Equal(want) {
		t.Fatalf("IpToHosts = %v, want %v", hosts, want)
	}
}
