package ip

import (
	"net/netip"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
)

func TestResolveHostsIPv4(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("127.0.0.1", "192.168.1.1")
	addrs := ResolveHosts(hosts)
	if addrs.Cardinality() != 2 {
		t.Fatalf("expected 2, got %d", addrs.Cardinality())
	}
	if !addrs.ContainsOne(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("missing 127.0.0.1")
	}
}

func TestResolveHostsIgnoresIPv6(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("::1", "127.0.0.1")
	addrs := ResolveHosts(hosts)
	if addrs.Cardinality() != 1 {
		t.Fatalf("expected 1, got %d", addrs.Cardinality())
	}
	if !addrs.ContainsOne(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("should contain 127.0.0.1")
	}
}

func TestResolveHostsViaCommonHostOrIp(t *testing.T) {
	// Use a hostname that common.HostOrIpToIpsSet can resolve? It may use DNS.
	// Instead use an IP that is not parseable as netip but is host? Actually HostOrIpToIpsSet handles hosts like "localhost" maybe?
	// We can test with an invalid host that returns empty set, should not add
	hosts := mapset.NewThreadUnsafeSet[string]("invalid_host_name_that_does_not_resolve_123")
	addrs := ResolveHosts(hosts)
	// Should be 0 because it fails to resolve and HostOrIpToIpsSet may return empty
	if addrs.Cardinality() != 0 {
		t.Fatalf("expected 0, got %d", addrs.Cardinality())
	}
}

func TestResolveHostsEmpty(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]()
	addrs := ResolveHosts(hosts)
	if addrs.Cardinality() != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestResolveHostsMixed(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("127.0.0.1", "::1", "10.0.0.1")
	addrs := ResolveHosts(hosts)
	if addrs.Cardinality() != 2 {
		t.Fatalf("expected 2 IPv4, got %d", addrs.Cardinality())
	}
}
