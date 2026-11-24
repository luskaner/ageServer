package ip

import (
	"net/netip"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
)

func TestResolveHostsIPv4(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("127.0.0.1", "192.168.1.1")
	addrs, errs := ResolveHosts(hosts)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if addrs.Cardinality() != 2 {
		t.Fatalf("expected 2, got %d", addrs.Cardinality())
	}
	if !addrs.ContainsOne(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("missing 127.0.0.1")
	}
}

func TestResolveHostsRejectsIPv6(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("::1", "127.0.0.1")
	addrs, errs := ResolveHosts(hosts)
	if len(errs) != 1 || errs[0].Host != "::1" {
		t.Fatalf("expected one error for ::1, got %v", errs)
	}
	if addrs.Cardinality() != 1 {
		t.Fatalf("expected 1, got %d", addrs.Cardinality())
	}
	if !addrs.ContainsOne(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("should contain 127.0.0.1")
	}
}

func TestResolveHostsUnresolvableHostIsAnError(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("invalid_host_name_that_does_not_resolve_123")
	addrs, errs := ResolveHosts(hosts)
	if addrs.Cardinality() != 0 {
		t.Fatalf("expected 0, got %d", addrs.Cardinality())
	}
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", errs)
	}
}

func TestResolveHostsEmpty(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]()
	addrs, errs := ResolveHosts(hosts)
	if addrs.Cardinality() != 0 || len(errs) != 0 {
		t.Fatal("empty should resolve to nothing")
	}
}

func TestResolveHostsMixed(t *testing.T) {
	hosts := mapset.NewThreadUnsafeSet[string]("127.0.0.1", "::1", "10.0.0.1")
	addrs, errs := ResolveHosts(hosts)
	if len(errs) != 1 {
		t.Fatalf("expected one IPv6 error, got %v", errs)
	}
	if addrs.Cardinality() != 2 {
		t.Fatalf("expected 2 IPv4, got %d", addrs.Cardinality())
	}
}
