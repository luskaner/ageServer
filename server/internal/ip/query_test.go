package ip

import (
	"net"
	"net/netip"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
)

type mockResolverIP struct {
	interfaces map[*net.Interface][]*net.IPNet
	err        error
}

func (m *mockResolverIP) HostToIPs(host string) []net.IP { return nil }
func (m *mockResolverIP) IPToHosts(ip string) []string   { return nil }
func (m *mockResolverIP) DirectHostToIP(host string) (string, error) {
	return "", nil
}
func (m *mockResolverIP) DialTCP(network, address string, timeout time.Duration) (net.Conn, error) {
	return nil, nil
}
func (m *mockResolverIP) NetInterfaces() ([]net.Interface, error) { return nil, nil }
func (m *mockResolverIP) RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error) {
	return m.interfaces, m.err
}

func TestQueryConnectionsNoMulticast(t *testing.T) {
	iface := &net.Interface{Index: 1, Name: "lo", Flags: net.FlagUp | net.FlagMulticast}
	ipNet := &net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}
	restore := common.SetResolver(&mockResolverIP{
		interfaces: map[*net.Interface][]*net.IPNet{
			iface: {ipNet},
		},
	})
	defer restore()
	addr := netip.MustParseAddr("127.0.0.1")
	groups := mapset.NewThreadUnsafeSet[netip.Addr]()
	err, conns := QueryConnections(addr, groups, 0)
	if err != nil {
		t.Fatalf("QueryConnections failed: %v", err)
	}
	if len(conns) == 0 {
		t.Fatal("expected at least one conn")
	}
	for _, c := range conns {
		c.Close()
	}
	// Test with unspecified IP
	addr2 := netip.IPv4Unspecified()
	err, conns2 := QueryConnections(addr2, groups, 0)
	if err != nil {
		t.Fatalf("unspecified failed: %v", err)
	}
	for _, c := range conns2 {
		c.Close()
	}
}

func TestQueryConnectionsErrorFromResolver(t *testing.T) {
	restore := common.SetResolver(&mockResolverIP{err: net.ErrClosed})
	defer restore()
	addr := netip.MustParseAddr("127.0.0.1")
	groups := mapset.NewThreadUnsafeSet[netip.Addr]()
	err, _ := QueryConnections(addr, groups, 0)
	if err == nil {
		t.Fatal("should fail when resolver fails")
	}
}

// localTestIPv4 returns an IPv4 address assigned to a local interface so
// net.ListenUDP can bind to it (binding to a non-local IP fails), preferring a
// non-loopback address and falling back to loopback.
func localTestIPv4(t *testing.T) net.IP {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	fallback := net.IPv4(127, 0, 0, 1)
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			if ip4.IsLoopback() {
				fallback = ip4
				continue
			}
			return ip4
		}
	}
	return fallback
}

func TestQueryConnectionsWithMulticast(t *testing.T) {
	localIP := localTestIPv4(t)
	iface := &net.Interface{Index: 1, Name: "eth0", Flags: net.FlagUp | net.FlagMulticast}
	ipNet := &net.IPNet{IP: localIP, Mask: net.CIDRMask(24, 32)}
	restore := common.SetResolver(&mockResolverIP{
		interfaces: map[*net.Interface][]*net.IPNet{
			iface: {ipNet},
		},
	})
	defer restore()
	addr, ok := netip.AddrFromSlice(localIP)
	if !ok {
		t.Fatal("failed to parse local IP")
	}
	addr = addr.Unmap()
	groups := mapset.NewThreadUnsafeSet[netip.Addr](netip.MustParseAddr("239.0.0.1"))
	err, conns := QueryConnections(addr, groups, 0)
	if err != nil {
		t.Fatalf("multicast failed: %v", err)
	}
	for _, c := range conns {
		c.Close()
	}
}

func TestQueryConnectionsNoMulticastFlagSkipped(t *testing.T) {
	// Interface without multicast flag should be skipped
	localIP := localTestIPv4(t)
	iface := &net.Interface{Index: 1, Name: "eth0", Flags: net.FlagUp} // no multicast
	ipNet := &net.IPNet{IP: localIP, Mask: net.CIDRMask(24, 32)}
	restore := common.SetResolver(&mockResolverIP{
		interfaces: map[*net.Interface][]*net.IPNet{
			iface: {ipNet},
		},
	})
	defer restore()
	addr, ok := netip.AddrFromSlice(localIP)
	if !ok {
		t.Fatal("failed to parse local IP")
	}
	addr = addr.Unmap()
	groups := mapset.NewThreadUnsafeSet[netip.Addr](netip.MustParseAddr("239.0.0.1"))
	err, conns := QueryConnections(addr, groups, 0)
	if err != nil {
		t.Fatalf("should not fail, got %v", err)
	}
	// Should still get a conn, but multicast group join should be skipped (no multicastIfs)
	for _, c := range conns {
		c.Close()
	}
}
