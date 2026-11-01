package cmdUtils

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/luskaner/ageServer/common/cmdlog"
)

var (
	listenTCP = func(address string) (error, net.Listener) {
		var addr *net.TCPAddr
		var err error
		addr, err = net.ResolveTCPAddr("tcp4", address)
		if err != nil {
			cmdlog.Fault("Could not resolve %s: %s", address, err)
			return err, nil
		}
		var listener *net.TCPListener
		listener, err = net.ListenTCP("tcp4", addr)
		if err != nil {
			cmdlog.Fault("Could not listen on %s: %s", address, err)
			return err, nil
		}
		return nil, listener
	}
	findUnusedPorts = func(count int) ([]int, error) {
		var ports []int
		var listeners []net.Listener
		address := net.JoinHostPort(netip.IPv4Unspecified().String(), "0")
		var err error
		for range count {
			var listener net.Listener
			err, listener = listenTCP(address)
			if err != nil {
				break
			}
			listeners = append(listeners, listener)
			ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
		}

		for _, l := range listeners {
			_ = l.Close()
		}

		if err != nil {
			return nil, err
		}

		return ports, nil
	}
)

func GeneratePortsAsNeeded(ports []int) (generatedPorts []int, err error) {
	var missingIndexes []int
	for i, port := range ports {
		if port == 0 {
			missingIndexes = append(missingIndexes, i)
		}
	}
	if len(missingIndexes) > 0 {
		cmdlog.Step("Generating ports...")
		generatedPorts, err = findUnusedPorts(len(missingIndexes))
		if err != nil {
			return nil, err
		}
		for idx, i := range missingIndexes {
			ports[i] = generatedPorts[idx]
		}
	}
	return ports, nil
}

func Available(port int) bool {
	address := net.JoinHostPort(netip.IPv4Unspecified().String(), fmt.Sprintf("%d", port))
	err, listener := listenTCP(address)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}
