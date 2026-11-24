package ip

import (
	"fmt"
	"net/netip"
	"slices"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
)

// ResolveError names the host that could not be turned into an IPv4 address.
// List is filled by the caller that owns the configuration section ("Modern" or
// "Legacy"), because the resolver does not know which list it was given.
type ResolveError struct {
	Host string
	List string
	Err  error
}

func (e *ResolveError) Error() string {
	if e.List != "" {
		return fmt.Sprintf("Hosts.%s: %s: %v", e.List, e.Host, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Host, e.Err)
}

func (e *ResolveError) Unwrap() error { return e.Err }

// ResolveHosts turns every configured host into an IPv4 address. IPv6 literals
// are rejected: the server only listens on IPv4 and a silently ignored address
// would look like a successful configuration.
func ResolveHosts(hosts mapset.Set[string]) (mapset.Set[netip.Addr], []ResolveError) {
	ipAddrs := mapset.NewThreadUnsafeSet[netip.Addr]()
	var errs []ResolveError
	hostList := hosts.ToSlice()
	slices.Sort(hostList)
	for _, host := range hostList {
		ipAddr, err := netip.ParseAddr(host)
		if err == nil {
			if !ipAddr.Is4() {
				errs = append(errs, ResolveError{Host: host, Err: fmt.Errorf("IPv6 addresses are not supported")})
				continue
			}
			ipAddrs.Add(ipAddr)
			continue
		}
		ips := common.HostOrIpToIpsSet(host)
		if ips.IsEmpty() {
			errs = append(errs, ResolveError{Host: host, Err: fmt.Errorf("could not resolve to an IPv4 address")})
			continue
		}
		ipList := ips.ToSlice()
		slices.Sort(ipList)
		for _, ip := range ipList {
			parsed, parseErr := netip.ParseAddr(ip)
			if parseErr != nil || !parsed.Is4() {
				continue
			}
			ipAddrs.Add(parsed)
		}
	}
	return ipAddrs, errs
}
