package battle_server_broadcast

import (
	"bytes"
	"net"

	"github.com/luskaner/ageServer/battle-server-broadcast/internal"
)

var Header = []byte{0x21, 0x24, 0x00}

const GuidLength = 36

const PortSize = 2

var MinimumSize = len(Header) + GuidLength + PortSize + 1 + 3*PortSize

type udpReadConn interface {
	ReadFromUDP(b []byte) (int, *net.UDPAddr, error)
	Close() error
}

type udpWriteConn interface {
	Write(b []byte) (int, error)
	Close() error
}

var (
	netInterfaces  = net.Interfaces
	interfaceAddrs = func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }
	netListenUDP   = func(network string, laddr *net.UDPAddr) (udpReadConn, error) {
		return net.ListenUDP(network, laddr)
	}
	netDialUDP = func(network string, laddr, raddr *net.UDPAddr) (udpWriteConn, error) {
		return net.DialUDP(network, laddr, raddr)
	}
)

func RetrieveBsInterfaceAddresses() (mostPriority *net.IPNet, restInterfaces []*net.IPNet, err error) {
	var interfaces []net.Interface
	interfaces, err = netInterfaces()
	if err != nil {
		return
	}

	for _, iface := range interfaces {
		addrs, localErr := interfaceAddrs(iface)
		if localErr != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet == nil {
				continue
			}
			if ipNet.IP == nil || ipNet.Mask == nil || ipNet.IP.To4() == nil {
				continue
			}
			// Ensure mask length matches IP length (IPv4 -> 4)
			if len(ipNet.Mask) != net.IPv4len && len(ipNet.Mask) != net.IPv6len {
				continue
			}
			if internal.FlagsCheck(iface.Flags) {
				if mostPriority == nil {
					mostPriority = ipNet
				} else {
					restInterfaces = append(restInterfaces, ipNet)
				}
			}
		}
	}
	return
}

func calculateBroadcastIp(ip net.IP, mask net.IPMask) net.IP {
	if ip == nil || mask == nil {
		return nil
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	if len(mask) != len(ip4) {
		// Normalize mask length: net.IPMask may be 4 or 16; try to get 4-byte form
		if len(mask) == net.IPv6len {
			mask = net.IPMask(mask[12:16])
		}
		if len(mask) != len(ip4) {
			return nil
		}
	}
	broadcast := make(net.IP, len(ip4))
	for i := range ip4 {
		broadcast[i] = ip4[i] | ^mask[i]
	}
	return broadcast
}

func ValidData(data []byte, length int) bool {
	return length >= MinimumSize && length <= len(data) && bytes.HasPrefix(data, Header)
}

func CloneAnnouncements(mostPriority *net.IPNet, restInterfaces []*net.IPNet, port int) (err error) {
	if mostPriority == nil || mostPriority.IP == nil {
		return nil
	}
	priorityUdpAddress := &net.UDPAddr{
		IP:   mostPriority.IP,
		Port: port,
	}

	var conn udpReadConn
	conn, err = netListenUDP("udp", priorityUdpAddress)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	var targets []udpWriteConn
	var lastDialErr error
	for _, restAddress := range restInterfaces {
		if restAddress == nil || restAddress.IP == nil || restAddress.Mask == nil {
			continue
		}
		broadcastIP := calculateBroadcastIp(restAddress.IP, restAddress.Mask)
		if broadcastIP == nil {
			continue
		}
		var restAddressConn udpWriteConn
		restAddressConn, lastDialErr = netDialUDP(
			"udp4",
			&net.UDPAddr{
				IP: restAddress.IP,
			},
			&net.UDPAddr{
				IP:   broadcastIP,
				Port: priorityUdpAddress.Port,
			},
		)
		if lastDialErr == nil && restAddressConn != nil {
			targets = append(targets, restAddressConn)
		}
	}

	if len(targets) == 0 {
		if lastDialErr != nil {
			err = lastDialErr
		}
		return
	}

	defer func() {
		for _, target := range targets {
			_ = target.Close()
		}
	}()

	// Extracted loop for testability
	return cloneAnnouncementsLoop(conn, mostPriority, targets)
}

// cloneAnnouncementsLoop is the packet forwarding loop, extracted for testing.
// It reads from conn and forwards valid packets from mostPriority to all targets.
func cloneAnnouncementsLoop(conn udpReadConn, mostPriority *net.IPNet, targets []udpWriteConn) error {
	buffer := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return err
		}
		if addr == nil || addr.IP == nil {
			continue
		}
		if !ValidData(buffer, n) || !addr.IP.Equal(mostPriority.IP) {
			continue
		}
		data := buffer[:n]
		for _, target := range targets {
			if target != nil {
				_, _ = target.Write(data)
			}
		}
	}
}
