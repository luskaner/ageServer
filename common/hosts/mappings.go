package hosts

import (
	"net"
	"strings"

	"github.com/luskaner/ageServer/common"
)

// HostMappings maps a host to an IP, but an IP can be mapped to multiple hosts.
type HostMappings map[Host]net.IP

func (h *HostMappings) Set(host Host, ip net.IP) {
	(*h)[host] = ip
}

func (h *HostMappings) Get(host Host) (ip net.IP, ok bool) {
	ip, ok = (*h)[host]
	return
}

func (h *HostMappings) Delete(host Host) {
	delete(*h, host)
}

func (h *HostMappings) String(lineEnding string) string {
	lines := make([]Line, len(*h))
	i := 0
	for host, ip := range *h {
		lines[i] = Line{ip: ip, hosts: []Host{host}}.WithOwnMarking()
		i++
	}
	sb := strings.Builder{}
	for _, line := range lines {
		sb.WriteString(lineEnding)
		sb.WriteString(line.String())
	}
	sb.WriteString(lineEnding)
	return sb.String()
}

func Mappings(gameId string, ip net.IP, withMacOsExclusive bool) HostMappings {
	mappings := make(HostMappings)
	if ip != nil {
		for _, host := range common.AllHosts(gameId, withMacOsExclusive) {
			mappings.Set(Host(host), ip)
		}
	}
	return mappings
}
