package common

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/miekg/dns"
)

// Google, Cloudfare and OpenDNS primaries then secondaries
var dnsServers = []string{"8.8.8.8", "1.1.1.1", "208.67.222.222", "8.8.4.4", "1.0.0.1", "208.67.220.220"}

var cacheTime = 1 * time.Minute
var failedIpToHosts map[string]time.Time
var failedHostToIps map[string]time.Time
var ipToHosts map[string]mapset.Set[string]
var hostToIps map[string]mapset.Set[string]

// Resolver abstracts DNS resolution so production uses real DNS and tests
// inject fakes. All methods are safe for concurrent use.
type Resolver interface {
	// HostToIPs resolves a hostname to IPv4 addresses.
	HostToIPs(host string) []net.IP
	// IPToHosts performs reverse DNS lookup on an IP address.
	IPToHosts(ip string) []string
	// DirectHostToIP queries external DNS servers directly for a host.
	DirectHostToIP(host string) (string, error)
	// DialTCP connects to addr with a timeout.
	DialTCP(network, address string, timeout time.Duration) (net.Conn, error)
	// NetInterfaces returns the OS network interfaces.
	NetInterfaces() ([]net.Interface, error)
	// RunningNetworkInterfaces returns up interfaces mapped to their IPv4 addrs.
	RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error)
}

// defaultResolver is the production implementation that calls real OS/DNS APIs.
type defaultResolver struct{}

func (r *defaultResolver) HostToIPs(host string) []net.IP {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ips, err := (&net.Resolver{}).LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil
	}
	return ips
}

func (r *defaultResolver) IPToHosts(ip string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	names, err := (&net.Resolver{}).LookupAddr(ctx, ip)
	if err != nil {
		return nil
	}
	return names
}

func (r *defaultResolver) DirectHostToIP(host string) (string, error) {
	fqdnHost := dns.Fqdn(host)
	m := new(dns.Msg)
	m.SetQuestion(fqdnHost, dns.TypeA)
	client := &dns.Client{Timeout: time.Second}
	for _, dnsServer := range dnsServers {
		in, _, err := client.Exchange(m, net.JoinHostPort(dnsServer, "53"))
		if err != nil {
			continue
		}
		if in.Rcode != dns.RcodeSuccess {
			continue
		}
		for _, ans := range in.Answer {
			if a, ok := ans.(*dns.A); ok {
				return a.A.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no IP found for %s", host)
}

func (r *defaultResolver) DialTCP(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

func (r *defaultResolver) NetInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (r *defaultResolver) RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make(map[*net.Interface][]*net.IPNet)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					if _, ok := result[&iface]; !ok {
						result[&iface] = make([]*net.IPNet, 0)
					}
					result[&iface] = append(result[&iface], ipnet)
				}
			}
		}
	}
	return result, nil
}

// deps holds the active resolver. Package-level functions delegate to it.
var deps Resolver = &defaultResolver{}

// SetResolver replaces the active resolver and returns a cleanup function
// that restores the previous one. Intended for tests:
//
//	cleanup := SetResolver(&mockResolver{})
//	defer cleanup()
func SetResolver(r Resolver) (restore func()) {
	orig := deps
	deps = r
	return func() { deps = orig }
}

func init() {
	ClearDNSCache()
}

func domainToIps(host string) []net.IP {
	return deps.HostToIPs(host)
}

func ipToDnsName(ip string) []string {
	return deps.IPToHosts(ip)
}

func cachedHostToIps(host string) (bool, mapset.Set[string]) {
	var cached bool
	var result mapset.Set[string]
	var cachedIps mapset.Set[string]
	hostToLower := strings.ToLower(host)
	if cachedIps, cached = hostToIps[hostToLower]; cached {
		result = cachedIps
	} else if failedTime, ok := failedHostToIps[hostToLower]; ok && time.Since(failedTime) < cacheTime {
		cached = true
	}
	return cached, result
}

func cachedIpToHosts(ip string) (bool, mapset.Set[string]) {
	var cached bool
	var result mapset.Set[string]
	var cachedHosts mapset.Set[string]
	if cachedHosts, cached = ipToHosts[ip]; cached {
		result = cachedHosts
	} else if failedTime, ok := failedIpToHosts[ip]; ok && time.Since(failedTime) < cacheTime {
		cached = true
	}
	return cached, result
}

func DirectHostToIP(host string) (string, error) {
	return deps.DirectHostToIP(host)
}

func DNSConnectivity() bool {
	for _, dnsServer := range dnsServers {
		conn, err := deps.DialTCP("tcp", net.JoinHostPort(dnsServer, "53"), time.Second)
		if err != nil {
			continue
		}
		if conn != nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

func CacheMapping(host string, ip string) {
	hostToLower := strings.ToLower(host)
	if _, exists := hostToIps[hostToLower]; !exists {
		hostToIps[hostToLower] = mapset.NewThreadUnsafeSet[string]()
	}
	hostToIps[hostToLower].Add(ip)
	if _, exists := ipToHosts[ip]; !exists {
		ipToHosts[ip] = mapset.NewThreadUnsafeSet[string]()
	}
	ipToHosts[ip].Add(host)
	if _, exists := failedIpToHosts[ip]; exists {
		delete(failedIpToHosts, ip)
	}
	if _, exists := failedHostToIps[hostToLower]; exists {
		delete(failedHostToIps, hostToLower)
	}
}

func ClearDNSCache() {
	failedIpToHosts = make(map[string]time.Time)
	failedHostToIps = make(map[string]time.Time)
	ipToHosts = make(map[string]mapset.Set[string])
	hostToIps = make(map[string]mapset.Set[string])
}

func HostOrIpToIps(host string) []string {
	if ip := net.ParseIP(host); ip != nil {
		var ips []string
		if ip.To4() != nil {
			if ip.IsUnspecified() {
				ips = append(ips, ResolveUnspecifiedIps()...)
			} else {
				ips = append(ips, ip.String())
			}
		}
		return ips
	}

	cached, cachedIps := cachedHostToIps(host)
	if cached {
		return cachedIps.Clone().ToSlice()
	}
	var ips []string
	ipsFromDns := domainToIps(host)
	if ipsFromDns != nil {
		for _, ipRaw := range ipsFromDns {
			ipStr := ipRaw.String()
			ips = append(ips, ipStr)
			CacheMapping(host, ipStr)
		}
	}
	return ips
}

func HostOrIpToIpsSet(host string) mapset.Set[string] {
	return mapset.NewSet[string](HostOrIpToIps(host)...)
}

func ResolveUnspecifiedIps() (ips []string) {
	interfaces, err := deps.NetInterfaces()

	if err != nil {
		return
	}

	var addrs []net.Addr
	for _, i := range interfaces {

		if i.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err = i.Addrs()
		if err != nil {
			return
		}

		for _, addr := range addrs {
			var currentIp net.IP
			v, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			currentIp = v.IP
			currentIpv4 := currentIp.To4()
			if currentIpv4 == nil {
				continue
			}

			ips = append(ips, currentIpv4.String())
		}
	}

	return
}

func Matches(addr1 string, addr2 string) bool {
	addr2Ips := HostOrIpToIpsSet(addr2)
	addr1Ips := HostOrIpToIpsSet(addr1)
	return !addr2Ips.Intersect(addr1Ips).IsEmpty()
}

func IpToHosts(ip string) mapset.Set[string] {
	cached, cachedHosts := cachedIpToHosts(ip)
	if cached {
		return cachedHosts
	}
	hosts := mapset.NewThreadUnsafeSet[string]()
	hostsFromDns := ipToDnsName(ip)
	if hostsFromDns != nil {
		for _, hostStr := range hostsFromDns {
			hosts.Add(hostStr)
			CacheMapping(hostStr, ip)
		}
	}
	return hosts
}
