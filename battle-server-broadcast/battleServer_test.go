package battle_server_broadcast

import (
	"net"
	"testing"
)

func TestHeaderAndMinimumSize(t *testing.T) {
	if len(Header) != 3 {
		t.Fatalf("Header length = %d, want 3", len(Header))
	}
	want := len(Header) + GuidLength + PortSize + 1 + 3*PortSize
	if MinimumSize != want {
		t.Fatalf("MinimumSize = %d, want %d", MinimumSize, want)
	}
}

func TestValidDataAcceptsWellFormedPacket(t *testing.T) {
	data := make([]byte, MinimumSize+4)
	copy(data, Header)
	if !ValidData(data, len(data)) {
		t.Fatal("well formed packet rejected")
	}
	// Exact minimum size must also pass.
	if !ValidData(data[:MinimumSize], MinimumSize) {
		t.Fatal("minimum sized packet rejected")
	}
}

func TestValidDataRejections(t *testing.T) {
	data := make([]byte, MinimumSize)
	copy(data, Header)

	if ValidData(data, MinimumSize-1) {
		t.Error("shorter than minimum must be rejected")
	}
	badHeader := make([]byte, MinimumSize)
	copy(badHeader, Header)
	badHeader[1] ^= 0xFF
	if ValidData(badHeader, len(badHeader)) {
		t.Error("wrong header must be rejected")
	}
	if ValidData(nil, MinimumSize) {
		t.Error("nil data must be rejected")
	}
}

// Regression: ValidData used to ignore that the declared length exceeded the
// actual data, letting callers slice data[:length] out of range.
func TestValidDataRejectsLengthBeyondData(t *testing.T) {
	short := make([]byte, MinimumSize-2)
	copy(short, Header)
	if ValidData(short, MinimumSize) {
		t.Fatal("declared length beyond data must be rejected")
	}
	if ValidData(Header, 1<<20) {
		t.Fatal("huge declared length over a tiny slice must be rejected")
	}
}

func TestCalculateBroadcastIp(t *testing.T) {
	ip := net.IPv4(192, 168, 1, 23).To4()
	mask := net.IPv4Mask(255, 255, 255, 0)
	got := calculateBroadcastIp(ip, mask)
	want := net.IPv4(192, 168, 1, 255).To4()
	if !got.Equal(want) {
		t.Fatalf("broadcast = %v, want %v", got, want)
	}
}

func TestCalculateBroadcastIpAllOnesMask(t *testing.T) {
	ip := net.IPv4(10, 0, 0, 1).To4()
	got := calculateBroadcastIp(ip, net.IPv4Mask(255, 255, 255, 255))
	if !got.Equal(ip) {
		t.Fatalf("host mask broadcast = %v, want %v", got, ip)
	}
}

// Integration-style smoke test over the real machine interfaces: a single
// interface whose Addrs() fails used to poison the named error return even
// when other interfaces provided valid results.
func TestRetrieveBsInterfaceAddressesCleanError(t *testing.T) {
	most, rest, err := RetrieveBsInterfaceAddresses()
	if err != nil {
		t.Fatalf("err = %v, want nil (interface-level failures must be skipped)", err)
	}
	for _, ipNet := range append(rest, most) {
		if ipNet != nil && ipNet.IP.To4() == nil {
			t.Fatalf("non-IPv4 network leaked: %v", ipNet)
		}
	}
}
