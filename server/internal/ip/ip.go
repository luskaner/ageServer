package ip

import (
	"net/netip"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
)

func ResolveHosts(hosts mapset.Set[string]) mapset.Set[netip.Addr] {
	ipAddrs := mapset.NewThreadUnsafeSet[netip.Addr]()
	for host := range hosts.Iter() {
		ipAddr, err := netip.ParseAddr(host)
		if err == nil && ipAddr.Is4() {
			ipAddrs.Add(ipAddr)
		} else if err != nil {
			ips := common.HostOrIpToIpsSet(host)
			for ip := range ips.Iter() {
				ipAddrs.Add(netip.MustParseAddr(ip))
			}
		}
	}
	return ipAddrs
}
