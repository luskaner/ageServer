package common

import (
	"errors"
	"net"
	"testing"
)

func TestNetInterfacesMock(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagUp, Name: "lo"}
	ipNet := &net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}
	defer SetResolver(&mockResolver{
		runningIfaces: func() (map[*net.Interface][]*net.IPNet, error) {
			return map[*net.Interface][]*net.IPNet{iface: {ipNet}}, nil
		},
	})()
	result, err := RunningNetworkInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(result))
	}
	for i := range result {
		if i.Name != "lo" {
			t.Fatalf("unexpected interface %q", i.Name)
		}
		if len(result[i]) != 1 || !result[i][0].IP.Equal(ipNet.IP) {
			t.Fatalf("unexpected IPNet: %v", result[i])
		}
	}
}

func TestNetInterfacesError(t *testing.T) {
	defer SetResolver(&mockResolver{})()
	_, err := RunningNetworkInterfaces()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunningNetworkInterfacesSkipsDown(t *testing.T) {
	defer SetResolver(&mockResolver{
		runningIfaces: func() (map[*net.Interface][]*net.IPNet, error) {
			up := &net.Interface{Flags: net.FlagUp, Name: "up0"}
			return map[*net.Interface][]*net.IPNet{
				up: {{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
			}, nil
		},
	})()
	result, err := RunningNetworkInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 interface (up only), got %d", len(result))
	}
}

func TestNetInterfacesFallback(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagUp, Name: "eth0"}
	ipNet := &net.IPNet{IP: net.IPv4(192, 168, 1, 100), Mask: net.CIDRMask(24, 32)}
	defer SetResolver(&mockResolver{
		netIfaces: func() ([]net.Interface, error) {
			return []net.Interface{*iface}, nil
		},
		runningIfaces: func() (map[*net.Interface][]*net.IPNet, error) {
			return map[*net.Interface][]*net.IPNet{iface: {ipNet}}, nil
		},
	})()
	result, err := RunningNetworkInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(result))
	}
}

func TestRunningIfacesMockError(t *testing.T) {
	defer SetResolver(&mockResolver{
		runningIfaces: func() (map[*net.Interface][]*net.IPNet, error) {
			return nil, errors.New("mock error")
		},
	})()
	_, err := RunningNetworkInterfaces()
	if err == nil {
		t.Fatal("expected error")
	}
}
