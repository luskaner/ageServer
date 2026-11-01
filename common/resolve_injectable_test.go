package common

import (
	"errors"
	"net"
	"testing"
	"time"
)

// mockResolver is a test double that implements Resolver without any I/O.
type mockResolver struct {
	hostToIPs     func(host string) []net.IP
	ipToHosts     func(ip string) []string
	directHost    func(host string) (string, error)
	dialTCP       func(network, address string, timeout time.Duration) (net.Conn, error)
	netIfaces     func() ([]net.Interface, error)
	runningIfaces func() (map[*net.Interface][]*net.IPNet, error)
}

func (m *mockResolver) HostToIPs(host string) []net.IP {
	if m.hostToIPs != nil {
		return m.hostToIPs(host)
	}
	return nil
}
func (m *mockResolver) IPToHosts(ip string) []string {
	if m.ipToHosts != nil {
		return m.ipToHosts(ip)
	}
	return nil
}
func (m *mockResolver) DirectHostToIP(host string) (string, error) {
	if m.directHost != nil {
		return m.directHost(host)
	}
	return "", errors.New("not implemented")
}
func (m *mockResolver) DialTCP(network, address string, timeout time.Duration) (net.Conn, error) {
	if m.dialTCP != nil {
		return m.dialTCP(network, address, timeout)
	}
	return nil, errors.New("not implemented")
}
func (m *mockResolver) NetInterfaces() ([]net.Interface, error) {
	if m.netIfaces != nil {
		return m.netIfaces()
	}
	return nil, errors.New("not implemented")
}
func (m *mockResolver) RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error) {
	if m.runningIfaces != nil {
		return m.runningIfaces()
	}
	return nil, errors.New("not implemented")
}

// mockConn satisfies net.Conn for dial mocks.
type mockConn struct{}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// --- Tests ---

func TestHostToIPsMock(t *testing.T) {
	defer SetResolver(&mockResolver{
		hostToIPs: func(host string) []net.IP {
			return []net.IP{net.ParseIP("1.2.3.4")}
		},
	})()
	ips := domainToIps("test.com")
	if len(ips) != 1 || ips[0].String() != "1.2.3.4" {
		t.Fatalf("got %v", ips)
	}
}

func TestHostToIPsNil(t *testing.T) {
	defer SetResolver(&mockResolver{})()
	if got := domainToIps("nohost.test"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestIPToHostsMock(t *testing.T) {
	defer SetResolver(&mockResolver{
		ipToHosts: func(ip string) []string {
			return []string{"host.test."}
		},
	})()
	names := ipToDnsName("1.2.3.4")
	if len(names) != 1 || names[0] != "host.test." {
		t.Fatalf("got %v", names)
	}
}

func TestDirectHostToIPMock(t *testing.T) {
	defer SetResolver(&mockResolver{
		directHost: func(host string) (string, error) {
			return "10.0.0.1", nil
		},
	})()
	ip, err := DirectHostToIP("test.com")
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.0.0.1" {
		t.Fatalf("got %q", ip)
	}
}

func TestDirectHostToIPError(t *testing.T) {
	defer SetResolver(&mockResolver{
		directHost: func(host string) (string, error) {
			return "", errors.New("no IP found")
		},
	})()
	_, err := DirectHostToIP("nonexistent.test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDNSConnectivityTrue(t *testing.T) {
	defer SetResolver(&mockResolver{
		dialTCP: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return &mockConn{}, nil
		},
	})()
	if !DNSConnectivity() {
		t.Fatal("expected true")
	}
}

func TestDNSConnectivityFalse(t *testing.T) {
	defer SetResolver(&mockResolver{
		dialTCP: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return nil, errors.New("refused")
		},
	})()
	if DNSConnectivity() {
		t.Fatal("expected false")
	}
}

func TestResolveUnspecifiedIpsMock(t *testing.T) {
	defer SetResolver(&mockResolver{
		netIfaces: func() ([]net.Interface, error) {
			return []net.Interface{{Flags: net.FlagUp, Name: "lo"}}, nil
		},
	})()
	_ = ResolveUnspecifiedIps()
}

func TestResolveUnspecifiedIpsError(t *testing.T) {
	defer SetResolver(&mockResolver{})()
	if got := ResolveUnspecifiedIps(); len(got) != 0 {
		t.Fatalf("expected empty on error, got %v", got)
	}
}

func TestHostOrIpToIpsCacheMiss(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()
	defer SetResolver(&mockResolver{
		hostToIPs: func(host string) []net.IP {
			if host == "resolved.test" {
				return []net.IP{net.ParseIP("198.51.100.1")}
			}
			return nil
		},
	})()

	ips := HostOrIpToIps("resolved.test")
	if len(ips) != 1 || ips[0] != "198.51.100.1" {
		t.Fatalf("got %v", ips)
	}
	if cached, _ := cachedHostToIps("resolved.test"); !cached {
		t.Fatal("expected cache hit")
	}
}

func TestIpToHostsCacheMiss(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()
	defer SetResolver(&mockResolver{
		ipToHosts: func(ip string) []string {
			if ip == "198.51.100.2" {
				return []string{"reverse.test."}
			}
			return nil
		},
	})()

	hosts := IpToHosts("198.51.100.2")
	if hosts == nil || !hosts.Contains("reverse.test.") {
		t.Fatalf("got %v", hosts)
	}
}

func TestCacheMappingDeletesFailedEntries(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	failedHostToIps["del.test"] = time.Now()
	failedIpToHosts["10.99.0.1"] = time.Now()
	CacheMapping("del.test", "10.99.0.1")

	if _, exists := failedHostToIps["del.test"]; exists {
		t.Error("failedHostToIps entry should be deleted")
	}
	if _, exists := failedIpToHosts["10.99.0.1"]; exists {
		t.Error("failedIpToHosts entry should be deleted")
	}
}

func TestCacheMappingCaseInsensitive(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	CacheMapping("UPPER.test", "10.0.0.1")
	if cached, _ := cachedHostToIps("upper.test"); !cached {
		t.Fatal("host lookup should be case-insensitive")
	}
}

func TestCachedHostToIpsFailedEntry(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	failedHostToIps["fail.test"] = time.Now()
	cached, result := cachedHostToIps("fail.test")
	if !cached {
		t.Fatal("recently failed host should return cached=true")
	}
	if result != nil {
		t.Fatal("failed cache should return nil result")
	}
}

func TestCachedIpToHostsFailedEntry(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	failedIpToHosts["10.99.0.2"] = time.Now()
	cached, result := cachedIpToHosts("10.99.0.2")
	if !cached {
		t.Fatal("recently failed IP should return cached=true")
	}
	if result != nil {
		t.Fatal("failed cache should return nil result")
	}
}

func TestCachedHostToIpsExpiredFailure(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	failedHostToIps["expired.test"] = time.Now().Add(-2 * time.Minute)
	cached, _ := cachedHostToIps("expired.test")
	if cached {
		t.Fatal("expired failure entry should not be cached")
	}
}

func TestCachedIpToHostsExpiredFailure(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()

	failedIpToHosts["10.99.0.3"] = time.Now().Add(-2 * time.Minute)
	cached, _ := cachedIpToHosts("10.99.0.3")
	if cached {
		t.Fatal("expired failure entry should not be cached")
	}
}

func TestMatchesWithDNS(t *testing.T) {
	defer SetResolver(&mockResolver{
		hostToIPs: func(host string) []net.IP {
			switch host {
			case "a.test":
				return []net.IP{net.ParseIP("10.0.0.1")}
			case "b.test":
				return []net.IP{net.ParseIP("10.0.0.1")}
			}
			return nil
		},
	})()
	if !Matches("a.test", "b.test") {
		t.Fatal("shared IP should match")
	}
}

func TestDefaultResolverHostToIPsLocalhost(t *testing.T) {
	r := &defaultResolver{}
	ips := r.HostToIPs("localhost")
	if len(ips) == 0 {
		t.Skip("localhost did not resolve, may be offline")
	}
	found := false
	for _, ip := range ips {
		if ip.String() == "127.0.0.1" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("localhost resolved to %v, not 127.0.0.1", ips)
	}
}

func TestDefaultResolverHostToIPsInvalid(t *testing.T) {
	r := &defaultResolver{}
	ips := r.HostToIPs("invalid.invalid")
	if ips != nil {
		t.Errorf("expected nil for invalid host, got %v", ips)
	}
}

func TestDefaultResolverIPToHostsLocalhost(t *testing.T) {
	r := &defaultResolver{}
	names := r.IPToHosts("127.0.0.1")
	_ = names
}

func TestDefaultResolverIPToHostsInvalid(t *testing.T) {
	r := &defaultResolver{}
	names := r.IPToHosts("999.999.999.999")
	if names != nil {
		t.Errorf("expected nil for invalid IP, got %v", names)
	}
}

func TestDefaultResolverDialTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	r := &defaultResolver{}
	conn, err := r.DialTCP("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("DialTCP: %v", err)
	}
	_ = conn.Close()
}

func TestDefaultResolverNetInterfaces(t *testing.T) {
	r := &defaultResolver{}
	ifaces, err := r.NetInterfaces()
	if err != nil {
		t.Fatalf("NetInterfaces: %v", err)
	}
	if len(ifaces) == 0 {
		t.Skip("no interfaces")
	}
}

func TestDefaultResolverRunningNetworkInterfaces(t *testing.T) {
	r := &defaultResolver{}
	m, err := r.RunningNetworkInterfaces()
	if err != nil {
		t.Fatalf("RunningNetworkInterfaces: %v", err)
	}
	_ = m
}

func TestHostOrIpToIpsUnspecified(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()
	ips := HostOrIpToIps("0.0.0.0")
	// Should return at least one local IP
	_ = ips
}

func TestHostOrIpToIpsDirectIP(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()
	ips := HostOrIpToIps("192.0.2.1")
	if len(ips) != 1 || ips[0] != "192.0.2.1" {
		t.Fatalf("got %v", ips)
	}
}

func TestHostOrIpToIpsCachedHit(t *testing.T) {
	ClearDNSCache()
	defer ClearDNSCache()
	CacheMapping("cached.test", "203.0.113.1")
	ips := HostOrIpToIps("cached.test")
	if len(ips) != 1 || ips[0] != "203.0.113.1" {
		t.Fatalf("cached hit failed: %v", ips)
	}
}

func TestDefaultResolverDirectHostToIPMocked(t *testing.T) {
	origServers := dnsServers
	dnsServers = []string{"127.0.0.1:0"} // invalid port, will fail quickly
	defer func() { dnsServers = origServers }()
	r := &defaultResolver{}
	_, err := r.DirectHostToIP("test.example.com")
	if err == nil {
		t.Error("expected error when DNS server unreachable")
	}
}
