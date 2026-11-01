package models

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luskaner/ageServer/server/internal"
)

func resetNetworkState(t *testing.T) {
	t.Helper()
	oldCanUseInternet := internal.CanUseInternet
	oldPublicIp := publicIp
	oldLocalSubnets := localSubnets
	t.Cleanup(func() {
		internal.CanUseInternet = oldCanUseInternet
		publicIp = oldPublicIp
		localSubnets = oldLocalSubnets
	})
	publicIp = ""
	localSubnets = nil
}

func autoBattleServer() *MainBattleServer {
	bs := &MainBattleServer{}
	bs.SetIPv4("auto")
	return bs
}

// localAddrRequest builds a request that carries the local address a real
// connection would: the address of this machine the peer reached us on. Every
// caller of ResolveIPv4 answers partly out of it, so a request without it can
// only ever prove that nothing is returned.
func localAddrRequest(localAddr string, remoteAddr string, host string) *http.Request {
	r := httptest.NewRequest("GET", "https://"+host+"/", nil)
	r.RemoteAddr = remoteAddr
	r.Host = host
	ctx := context.WithValue(r.Context(), http.LocalAddrContextKey, mustTCPAddr(localAddr))
	return r.WithContext(ctx)
}

func mustTCPAddr(addr string) *net.TCPAddr {
	ip, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		panic(err)
	}
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

func singleSubnet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	return subnet
}

// Regression: the subnets were only cached once a public address had been
// resolved, which left the list empty whenever the lookup was blocked, slow or
// answered by a captive portal. Every peer, the ones on the local networks
// included, was then mistaken for a remote one and handed a public address, or
// none at all, instead of the address they had just used.
func TestCacheNetworkInterfacesCachesSubnetsWithoutPublicIP(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = false
	cacheLocalSubnets()
	if len(localSubnets) == 0 {
		t.Fatal("localSubnets should not depend on the public IP lookup")
	}
}

func TestCacheNetworkInterfacesNoInternet(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = false

	CacheNetworkInterfaces("")
	if publicIp != "" {
		t.Fatalf("publicIp = %q, want empty without internet", publicIp)
	}
}

func TestCacheNetworkInterfacesAutoOffline(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = false

	CacheNetworkInterfaces("auto")
	if publicIp != "" {
		t.Fatalf("publicIp = %q, want empty with 'auto' and no internet", publicIp)
	}
}

func TestCacheNetworkInterfacesExternalIP(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = false

	CacheNetworkInterfaces("1.2.3.4")
	if publicIp != "1.2.3.4" {
		t.Fatalf("publicIp = %q, want 1.2.3.4 from external IP", publicIp)
	}
	if len(localSubnets) == 0 {
		t.Fatal("localSubnets should be populated when a public IP is available")
	}
}

func TestResolveIPv4OfflineNoPublicFallback(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = false
	publicIp = "8.8.8.8"
	localSubnets = []*net.IPNet{singleSubnet(t, "192.168.1.0/24")}

	bs := autoBattleServer()
	req := localAddrRequest("192.168.1.10:443", "9.9.9.9:5555", "relic-link.com")
	if ip := bs.ResolveIPv4(req); ip != "192.168.1.10" {
		t.Fatalf("ResolveIPv4 = %q, want the local address (no public IP leak offline)", ip)
	}
}

func TestResolveIPv4OnlinePublicFallback(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = true
	publicIp = "8.8.8.8"
	localSubnets = []*net.IPNet{singleSubnet(t, "192.168.1.0/24")}

	bs := autoBattleServer()

	req := localAddrRequest("192.168.1.10:443", "9.9.9.9:5555", "cdn.example.com")
	if ip := bs.ResolveIPv4(req); ip != "8.8.8.8" {
		t.Fatalf("ResolveIPv4 = %q, want 8.8.8.8 public fallback", ip)
	}

	req = localAddrRequest("192.168.1.10:443", "9.9.9.9:5555", "10.1.2.3")
	if ip := bs.ResolveIPv4(req); ip != "10.1.2.3" {
		t.Fatalf("ResolveIPv4 = %q, want 10.1.2.3 from host", ip)
	}

	req = localAddrRequest("192.168.1.10:443", "192.168.1.50:5555", "cdn.example.com")
	if ip := bs.ResolveIPv4(req); ip != "192.168.1.10" {
		t.Fatalf("ResolveIPv4 = %q, want the local address for a LAN peer", ip)
	}
}

// Regression: a peer on a different, still private, subnet reaches us through a
// router, a second access point or a guest network. It is not on any of our own
// subnets, but it did not come from the public internet either, so handing it
// our public address left it unable to connect.
func TestResolveIPv4CrossSubnetLanPeer(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = true
	publicIp = "8.8.8.8"
	localSubnets = []*net.IPNet{singleSubnet(t, "192.168.1.0/24")}

	bs := autoBattleServer()
	req := localAddrRequest("192.168.1.10:443", "192.168.2.50:5555", "cdn.example.com")
	if ip := bs.ResolveIPv4(req); ip != "192.168.1.10" {
		t.Fatalf("ResolveIPv4 = %q, want the local address for a cross-subnet LAN peer", ip)
	}
}

func TestResolveIPv4NotRoutablePeers(t *testing.T) {
	for _, remote := range []string{
		"127.0.0.1",
		"10.4.5.6",
		"172.31.255.254",
		"192.168.50.4",
		"100.100.5.5",
		"169.254.10.10",
		"224.0.0.251",
		"0.0.0.0",
	} {
		t.Run(remote, func(t *testing.T) {
			resetNetworkState(t)
			internal.CanUseInternet = true
			publicIp = "8.8.8.8"
			localSubnets = nil

			bs := autoBattleServer()
			req := localAddrRequest("192.168.1.10:443", net.JoinHostPort(remote, "5555"), "cdn.example.com")
			if ip := bs.ResolveIPv4(req); ip != "192.168.1.10" {
				t.Fatalf("ResolveIPv4 = %q, want the local address for %s", ip, remote)
			}
		})
	}
}

// Regression: an empty address is never usable. When the public lookup failed
// there was nothing left to answer a remote peer with, and the client was left
// waiting on a battle server at no address at all.
func TestResolveIPv4NeverReturnsEmpty(t *testing.T) {
	resetNetworkState(t)
	internal.CanUseInternet = true
	publicIp = ""
	localSubnets = []*net.IPNet{singleSubnet(t, "192.168.1.0/24")}

	bs := autoBattleServer()
	req := localAddrRequest("192.168.1.10:443", "9.9.9.9:5555", "cdn.example.com")
	if ip := bs.ResolveIPv4(req); ip != "192.168.1.10" {
		t.Fatalf("ResolveIPv4 = %q, want the local address rather than none", ip)
	}
}

func TestIsNotRoutable(t *testing.T) {
	notRoutable := []string{
		"0.0.0.0",
		"10.0.0.1",
		"100.64.0.1",
		"100.127.255.254",
		"127.0.0.1",
		"169.254.1.1",
		"172.16.0.1",
		"192.168.0.1",
		"224.0.0.251",
		"239.255.255.250",
		"255.255.255.255",
	}
	routable := []string{
		"1.1.1.1",
		"8.8.8.8",
		"9.9.9.9",
		"100.63.255.255",
		"100.128.0.1",
		"172.15.255.255",
		"172.32.0.1",
		"192.167.255.255",
		"192.169.0.1",
		"223.255.255.255",
	}
	for _, ip := range notRoutable {
		if !isNotRoutable(net.ParseIP(ip).To4()) {
			t.Errorf("isNotRoutable(%s) = false, want true", ip)
		}
	}
	for _, ip := range routable {
		if isNotRoutable(net.ParseIP(ip).To4()) {
			t.Errorf("isNotRoutable(%s) = true, want false", ip)
		}
	}
}
