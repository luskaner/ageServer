package battle_server_broadcast

import (
	"errors"
	"net"
	"testing"

	"github.com/luskaner/ageServer/battle-server-broadcast/internal"
)

// Helpers for mocking net
func restoreNetMocks(origInterfaces func() ([]net.Interface, error), origAddrs func(net.Interface) ([]net.Addr, error), origListen func(string, *net.UDPAddr) (udpReadConn, error), origDial func(string, *net.UDPAddr, *net.UDPAddr) (udpWriteConn, error)) {
	netInterfaces = origInterfaces
	interfaceAddrs = origAddrs
	netListenUDP = origListen
	netDialUDP = origDial
}

func TestRetrieveBsInterfaceAddresses_MockedSuccess(t *testing.T) {
	origInterfaces := netInterfaces
	origAddrs := interfaceAddrs
	defer restoreNetMocks(origInterfaces, origAddrs, netListenUDP, netDialUDP)

	// Mock interfaces: first is up+broadcast, second is up+broadcast, third is down
	iface1 := net.Interface{Name: "eth0", Flags: net.FlagUp | net.FlagBroadcast}
	iface2 := net.Interface{Name: "eth1", Flags: net.FlagUp | net.FlagBroadcast}
	iface3 := net.Interface{Name: "eth2", Flags: 0} // not up

	ipNet1 := &net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}
	ipNet2 := &net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(16, 32)}
	ipNet3 := &net.IPNet{IP: net.ParseIP("172.16.0.1"), Mask: net.CIDRMask(24, 32)}

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{iface1, iface2, iface3}, nil
	}
	interfaceAddrs = func(i net.Interface) ([]net.Addr, error) {
		switch i.Name {
		case "eth0":
			return []net.Addr{ipNet1}, nil
		case "eth1":
			return []net.Addr{ipNet2}, nil
		case "eth2":
			return []net.Addr{ipNet3}, nil
		default:
			return nil, nil
		}
	}

	most, rest, err := RetrieveBsInterfaceAddresses()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if most == nil || !most.IP.Equal(ipNet1.IP) {
		t.Fatalf("mostPriority = %v, want %v", most, ipNet1)
	}
	if len(rest) != 1 || !rest[0].IP.Equal(ipNet2.IP) {
		t.Fatalf("rest = %v, want [%v]", rest, ipNet2)
	}
}

func TestRetrieveBsInterfaceAddresses_NetInterfacesError(t *testing.T) {
	origInterfaces := netInterfaces
	origAddrs := interfaceAddrs
	defer restoreNetMocks(origInterfaces, origAddrs, netListenUDP, netDialUDP)

	netInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("interfaces fail")
	}
	_, _, err := RetrieveBsInterfaceAddresses()
	if err == nil || err.Error() != "interfaces fail" {
		t.Fatalf("expected interfaces error, got %v", err)
	}
}

func TestRetrieveBsInterfaceAddresses_AddrsErrorSkipped(t *testing.T) {
	origInterfaces := netInterfaces
	origAddrs := interfaceAddrs
	defer restoreNetMocks(origInterfaces, origAddrs, netListenUDP, netDialUDP)

	ifaceBad := net.Interface{Name: "bad", Flags: net.FlagUp | net.FlagBroadcast}
	ifaceGood := net.Interface{Name: "good", Flags: net.FlagUp | net.FlagBroadcast}
	ipNetGood := &net.IPNet{IP: net.ParseIP("192.168.1.20"), Mask: net.CIDRMask(24, 32)}

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{ifaceBad, ifaceGood}, nil
	}
	interfaceAddrs = func(i net.Interface) ([]net.Addr, error) {
		if i.Name == "bad" {
			return nil, errors.New("addrs fail")
		}
		return []net.Addr{ipNetGood}, nil
	}

	most, rest, err := RetrieveBsInterfaceAddresses()
	if err != nil {
		t.Fatalf("err should be nil when one interface fails but other succeeds, got %v", err)
	}
	if most == nil || !most.IP.Equal(ipNetGood.IP) {
		t.Fatalf("most should be good interface, got %v", most)
	}
	if len(rest) != 0 {
		t.Fatalf("rest should be empty, got %v", rest)
	}
}

func TestRetrieveBsInterfaceAddresses_SkipsNonIPNetAndInvalid(t *testing.T) {
	origInterfaces := netInterfaces
	origAddrs := interfaceAddrs
	defer restoreNetMocks(origInterfaces, origAddrs, netListenUDP, netDialUDP)

	iface := net.Interface{Name: "eth0", Flags: net.FlagUp | net.FlagBroadcast}
	// Non-IPNet addr, nil IP, nil Mask, IPv6, bad mask length
	nonIPNet := &net.IPAddr{IP: net.ParseIP("192.168.1.1")}
	nilIPNet := &net.IPNet{IP: nil, Mask: net.CIDRMask(24, 32)}
	nilMask := &net.IPNet{IP: net.ParseIP("192.168.1.2"), Mask: nil}
	ipv6Net := &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}
	// Valid one
	valid := &net.IPNet{IP: net.ParseIP("192.168.1.99"), Mask: net.CIDRMask(24, 32)}
	// Invalid mask length
	invalidMask := &net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.IPMask{255, 255}} // len 2

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{iface}, nil
	}
	interfaceAddrs = func(i net.Interface) ([]net.Addr, error) {
		return []net.Addr{nonIPNet, nilIPNet, nilMask, ipv6Net, invalidMask, valid}, nil
	}

	most, rest, err := RetrieveBsInterfaceAddresses()
	if err != nil {
		t.Fatal(err)
	}
	if most == nil || !most.IP.Equal(valid.IP) {
		t.Fatalf("should only pick valid, got most=%v", most)
	}
	if len(rest) != 0 {
		t.Fatalf("rest should be 0, got %d", len(rest))
	}
}

func TestRetrieveBsInterfaceAddresses_FlagsFiltering(t *testing.T) {
	origInterfaces := netInterfaces
	origAddrs := interfaceAddrs
	defer restoreNetMocks(origInterfaces, origAddrs, netListenUDP, netDialUDP)

	ifaceUpBroadcast := net.Interface{Name: "a", Flags: net.FlagUp | net.FlagBroadcast}
	ifaceLoopback := net.Interface{Name: "lo", Flags: net.FlagUp | net.FlagBroadcast | net.FlagLoopback}
	ifaceDown := net.Interface{Name: "down", Flags: net.FlagBroadcast}
	ip1 := &net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(24, 32)}
	ip2 := &net.IPNet{IP: net.ParseIP("10.0.0.2"), Mask: net.CIDRMask(24, 32)}
	ip3 := &net.IPNet{IP: net.ParseIP("10.0.0.3"), Mask: net.CIDRMask(24, 32)}

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{ifaceUpBroadcast, ifaceLoopback, ifaceDown}, nil
	}
	interfaceAddrs = func(i net.Interface) ([]net.Addr, error) {
		switch i.Name {
		case "a":
			return []net.Addr{ip1}, nil
		case "lo":
			return []net.Addr{ip2}, nil
		case "down":
			return []net.Addr{ip3}, nil
		}
		return nil, nil
	}

	most, rest, _ := RetrieveBsInterfaceAddresses()
	if most == nil || !most.IP.Equal(ip1.IP) {
		t.Fatalf("only up+broadcast should be selected, got %v", most)
	}
	if len(rest) != 0 {
		t.Fatalf("rest should be 0, got %v", rest)
	}
	// Ensure FlagsCheck works as expected
	if !internal.FlagsCheck(ifaceUpBroadcast.Flags) {
		t.Error("up+broadcast should pass")
	}
	if internal.FlagsCheck(ifaceLoopback.Flags) {
		t.Error("loopback should be excluded")
	}
}

func TestCalculateBroadcastIp_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		mask net.IPMask
		want net.IP
	}{
		{"nil ip", nil, net.CIDRMask(24, 32), nil},
		{"nil mask", net.ParseIP("192.168.1.1"), nil, nil},
		{"ipv6", net.ParseIP("fe80::1"), net.CIDRMask(64, 128), nil},
		{"mismatched mask len 2", net.ParseIP("192.168.1.1").To4(), net.IPMask{255, 255}, nil},
		{"valid /24", net.ParseIP("192.168.1.10").To4(), net.CIDRMask(24, 32), net.ParseIP("192.168.1.255").To4()},
		{"valid /16", net.ParseIP("10.0.1.5").To4(), net.CIDRMask(16, 32), net.ParseIP("10.0.255.255").To4()},
		{"host mask", net.ParseIP("10.0.0.1").To4(), net.CIDRMask(32, 32), net.ParseIP("10.0.0.1").To4()},
		{"zero mask", net.ParseIP("192.168.1.10").To4(), net.CIDRMask(0, 32), net.ParseIP("255.255.255.255").To4()},
		{"16-byte mask valid", net.ParseIP("192.168.1.10").To4(), net.IPMask(append(make([]byte, 12), 255, 255, 255, 0)), net.ParseIP("192.168.1.255").To4()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateBroadcastIp(tc.ip, tc.mask)
			if tc.want == nil && got != nil {
				t.Fatalf("expected nil, got %v", got)
			}
			if tc.want != nil && !got.Equal(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestValidData_Table(t *testing.T) {
	// Valid header + minimum size
	valid := make([]byte, MinimumSize)
	copy(valid, Header)

	cases := []struct {
		name   string
		data   []byte
		length int
		want   bool
	}{
		{"exact minimum", valid, MinimumSize, true},
		{"longer than minimum", append(valid, 0x00), MinimumSize + 1, true},
		{"length beyond data", valid, MinimumSize + 1, false},
		{"nil data", nil, MinimumSize, false},
		{"empty data", []byte{}, 0, false},
		{"short length", valid, MinimumSize - 1, false},
		{"wrong header", func() []byte { b := make([]byte, MinimumSize); b[0] = 0xFF; copy(b[1:], Header[1:]); return b }(), MinimumSize, false},
		{"negative length", valid, -1, false},
		{"huge length", valid, 1 << 20, false},
		{"length equals data len but header ok", valid, len(valid), true},
		{"data longer than length header ok", append(valid, make([]byte, 10)...), MinimumSize, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidData(tc.data, tc.length); got != tc.want {
				t.Errorf("ValidData(%v, %d) = %v want %v", tc.data, tc.length, got, tc.want)
			}
		})
	}
}

// --- Tests for CloneAnnouncements ---

func TestCloneAnnouncements_NilMostPriority(t *testing.T) {
	origListen := netListenUDP
	origDial := netDialUDP
	defer restoreNetMocks(netInterfaces, interfaceAddrs, origListen, origDial)

	// Should not panic, return nil
	if err := CloneAnnouncements(nil, nil, 1234); err != nil {
		t.Fatalf("nil mostPriority should return nil, got %v", err)
	}
	if err := CloneAnnouncements(&net.IPNet{IP: nil}, nil, 1234); err != nil {
		t.Fatalf("nil IP should return nil, got %v", err)
	}
}

func TestCloneAnnouncements_ListenError(t *testing.T) {
	origListen := netListenUDP
	origDial := netDialUDP
	defer restoreNetMocks(netInterfaces, interfaceAddrs, origListen, origDial)

	netListenUDP = func(network string, laddr *net.UDPAddr) (udpReadConn, error) {
		return nil, errors.New("listen fail")
	}
	most := &net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}
	err := CloneAnnouncements(most, []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(24, 32)}}, 1234)
	if err == nil || err.Error() != "listen fail" {
		t.Fatalf("expected listen error, got %v", err)
	}
}

func TestCloneAnnouncements_NoTargets(t *testing.T) {
	origListen := netListenUDP
	origDial := netDialUDP
	defer restoreNetMocks(netInterfaces, interfaceAddrs, origListen, origDial)

	// Mock listen success but dial fails so no targets
	netListenUDP = func(network string, laddr *net.UDPAddr) (udpReadConn, error) {
		return &fakeReadConn{}, nil
	}
	netDialUDP = func(network string, laddr, raddr *net.UDPAddr) (udpWriteConn, error) {
		return nil, errors.New("dial fail")
	}
	most := &net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}
	rest := []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(24, 32)}}
	err := CloneAnnouncements(most, rest, 1234)
	if err == nil || err.Error() != "dial fail" {
		t.Fatalf("expected dial error when no targets, got %v", err)
	}

	// Empty rest should return nil without error
	err = CloneAnnouncements(most, nil, 1234)
	if err != nil {
		t.Fatalf("empty rest should return nil, got %v", err)
	}
	// Nil entries and invalid broadcast should be skipped, resulting in no targets
	invalidRest := []*net.IPNet{{IP: nil, Mask: net.CIDRMask(24, 32)}, {IP: net.ParseIP("192.168.1.1"), Mask: nil}}
	err = CloneAnnouncements(most, invalidRest, 1234)
	if err != nil {
		// Should return nil (no dial attempted, lastDialErr remains nil)
		t.Fatalf("invalid rest should be skipped and return nil, got %v", err)
	}
}

func TestCloneAnnouncements_SuccessCreatesTargetsAndLoops(t *testing.T) {
	origListen := netListenUDP
	origDial := netDialUDP
	defer restoreNetMocks(netInterfaces, interfaceAddrs, origListen, origDial)

	most := &net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}
	rest := []*net.IPNet{
		{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(24, 32)},
		{IP: net.ParseIP("10.0.0.6"), Mask: net.CIDRMask(24, 32)},
	}

	// Track dial calls
	dialCount := 0
	netListenUDP = func(network string, laddr *net.UDPAddr) (udpReadConn, error) {
		if laddr.IP.String() != most.IP.String() {
			t.Errorf("listen IP %v want %v", laddr.IP, most.IP)
		}
		// Return fake that will error immediately to break loop
		return &fakeReadConn{packets: []fakePacket{{err: errors.New("stop loop")}}}, nil
	}
	netDialUDP = func(network string, laddr, raddr *net.UDPAddr) (udpWriteConn, error) {
		dialCount++
		// Verify broadcast calculation: 10.0.0.5/24 -> 10.0.0.255
		expectedBcast := calculateBroadcastIp(laddr.IP, rest[dialCount-1].Mask)
		if !raddr.IP.Equal(expectedBcast) {
			t.Errorf("broadcast IP %v want %v", raddr.IP, expectedBcast)
		}
		return &fakeWriteConn{}, nil
	}

	err := CloneAnnouncements(most, rest, 5678)
	if err == nil || err.Error() != "stop loop" {
		t.Fatalf("expected stop loop error from Read, got %v", err)
	}
	if dialCount != 2 {
		t.Fatalf("expected 2 dials, got %d", dialCount)
	}
}

// --- Tests for cloneAnnouncementsLoop directly ---

type fakeReadConn struct {
	packets []fakePacket
	idx     int
	closed  bool
}

type fakePacket struct {
	data []byte
	addr *net.UDPAddr
	err  error
}

func (f *fakeReadConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	if f.idx >= len(f.packets) {
		return 0, nil, errors.New("no more packets")
	}
	p := f.packets[f.idx]
	f.idx++
	if p.err != nil {
		return 0, nil, p.err
	}
	n := copy(b, p.data)
	return n, p.addr, nil
}
func (f *fakeReadConn) Close() error { f.closed = true; return nil }

type fakeWriteConn struct {
	writes [][]byte
	closed bool
	err    error
}

func (f *fakeWriteConn) Write(b []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	f.writes = append(f.writes, cp)
	return len(b), nil
}
func (f *fakeWriteConn) Close() error { f.closed = true; return nil }

func TestCloneAnnouncementsLoop_ForwardsValidPackets(t *testing.T) {
	mostIP := net.ParseIP("192.168.1.10")
	most := &net.IPNet{IP: mostIP, Mask: net.CIDRMask(24, 32)}

	validData := make([]byte, MinimumSize)
	copy(validData, Header)
	// Fill rest with dummy
	for i := len(Header); i < MinimumSize; i++ {
		validData[i] = byte(i)
	}
	invalidHeader := make([]byte, MinimumSize)
	copy(invalidHeader, Header)
	invalidHeader[0] ^= 0xFF
	shortData := make([]byte, MinimumSize-1)
	copy(shortData, Header)

	readConn := &fakeReadConn{
		packets: []fakePacket{
			{data: validData, addr: &net.UDPAddr{IP: mostIP}},                         // should forward
			{data: invalidHeader, addr: &net.UDPAddr{IP: mostIP}},                     // wrong header -> skip
			{data: shortData, addr: &net.UDPAddr{IP: mostIP}},                         // too short -> skip (ValidData will reject because n < MinimumSize)
			{data: validData, addr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}},         // wrong IP -> skip
			{data: validData, addr: nil},                                              // nil addr -> skip
			{data: validData, addr: &net.UDPAddr{IP: nil}},                            // nil IP -> skip
			{data: validData, addr: &net.UDPAddr{IP: mostIP}},                         // should forward again
			{err: errors.New("stop")},                                                // should return error
		},
	}
	write1 := &fakeWriteConn{}
	write2 := &fakeWriteConn{}
	targets := []udpWriteConn{write1, write2, nil} // include nil to test skip

	err := cloneAnnouncementsLoop(readConn, most, targets)
	if err == nil || err.Error() != "stop" {
		t.Fatalf("expected stop error, got %v", err)
	}
	// Should have forwarded exactly 2 packets (first and 7th)
	if len(write1.writes) != 2 {
		t.Fatalf("write1 expected 2 writes, got %d", len(write1.writes))
	}
	if len(write2.writes) != 2 {
		t.Fatalf("write2 expected 2 writes, got %d", len(write2.writes))
	}
	for _, w := range write1.writes {
		if !ValidData(w, len(w)) {
			t.Error("forwarded data should be valid")
		}
	}
}

func TestCloneAnnouncementsLoop_ReadErrorReturns(t *testing.T) {
	most := &net.IPNet{IP: net.ParseIP("192.168.1.10")}
	readConn := &fakeReadConn{packets: []fakePacket{{err: errors.New("read fail")}}}
	err := cloneAnnouncementsLoop(readConn, most, []udpWriteConn{&fakeWriteConn{}})
	if err == nil || err.Error() != "read fail" {
		t.Fatalf("expected read fail, got %v", err)
	}
}

func TestCloneAnnouncementsLoop_WriteErrorIgnored(t *testing.T) {
	mostIP := net.ParseIP("192.168.1.10")
	most := &net.IPNet{IP: mostIP}
	validData := make([]byte, MinimumSize)
	copy(validData, Header)
	readConn := &fakeReadConn{
		packets: []fakePacket{
			{data: validData, addr: &net.UDPAddr{IP: mostIP}},
			{err: errors.New("stop")},
		},
	}
	// One writer fails, other succeeds
	failWriter := &fakeWriteConn{err: errors.New("write fail")}
	okWriter := &fakeWriteConn{}
	err := cloneAnnouncementsLoop(readConn, most, []udpWriteConn{failWriter, okWriter})
	if err == nil || err.Error() != "stop" {
		t.Fatalf("expected stop, got %v", err)
	}
	if len(okWriter.writes) != 1 {
		t.Fatalf("ok writer should have 1 write, got %d", len(okWriter.writes))
	}
	if len(failWriter.writes) != 0 {
		t.Fatalf("fail writer should have 0, got %d", len(failWriter.writes))
	}
}
