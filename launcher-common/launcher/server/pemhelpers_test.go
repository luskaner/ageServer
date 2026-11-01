package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/netip"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
)

func generatePEMCert(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestParseCertFromPEMValid(t *testing.T) {
	pemBytes := generatePEMCert(t, time.Now().Add(time.Hour))
	cert := parseCertFromPEM(pemBytes)
	if cert == nil {
		t.Fatal("expected valid certificate")
	}
	if cert.Subject.CommonName != "test" {
		t.Errorf("CommonName = %q, want %q", cert.Subject.CommonName, "test")
	}
}

func TestParseCertFromPEMNilBlock(t *testing.T) {
	cert := parseCertFromPEM([]byte("not a pem"))
	if cert != nil {
		t.Fatal("expected nil for non-PEM input")
	}
}

func TestParseCertFromPEMWrongBlockType(t *testing.T) {
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("data")}
	pemBytes := pem.EncodeToMemory(block)
	cert := parseCertFromPEM(pemBytes)
	if cert != nil {
		t.Fatal("expected nil for wrong block type")
	}
}

func TestParseCertFromPEMInvalidDER(t *testing.T) {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid der")}
	pemBytes := pem.EncodeToMemory(block)
	cert := parseCertFromPEM(pemBytes)
	if cert != nil {
		t.Fatal("expected nil for invalid DER data")
	}
}

func TestParseCertFromPEMEmptyInput(t *testing.T) {
	cert := parseCertFromPEM(nil)
	if cert != nil {
		t.Fatal("expected nil for nil input")
	}
	cert = parseCertFromPEM([]byte{})
	if cert != nil {
		t.Fatal("expected nil for empty input")
	}
}

func TestCertSoonExpiredFromBytesExpired(t *testing.T) {
	pemBytes := generatePEMCert(t, time.Now().Add(-time.Hour))
	if !certSoonExpiredFromBytes(pemBytes, time.Now()) {
		t.Error("certificate already expired, should return true")
	}
}

func TestCertSoonExpiredFromBytesSoonExpired(t *testing.T) {
	pemBytes := generatePEMCert(t, time.Now().Add(12*time.Hour))
	if !certSoonExpiredFromBytes(pemBytes, time.Now()) {
		t.Error("certificate expires in 12h, should return true (within 24h)")
	}
}

func TestCertSoonExpiredFromBytesNotExpired(t *testing.T) {
	pemBytes := generatePEMCert(t, time.Now().Add(72*time.Hour))
	if certSoonExpiredFromBytes(pemBytes, time.Now()) {
		t.Error("certificate expires in 72h, should return false")
	}
}

func TestCertSoonExpiredFromBytesJustBeforeExpiry(t *testing.T) {
	pemBytes := generatePEMCert(t, time.Now().Add(25*time.Hour))
	if certSoonExpiredFromBytes(pemBytes, time.Now()) {
		t.Error("certificate expires in 25h, should return false (beyond 24h threshold)")
	}
}

func TestCertSoonExpiredFromBytesAtBoundary(t *testing.T) {
	now := time.Now()
	// Give a small buffer so ASN.1 time serialization doesn't shift the boundary
	certExpiry := now.Add(24 * time.Hour).Add(time.Second)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     certExpiry,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	// now + 24h < NotAfter (by 1 second) → should NOT be expired
	if certSoonExpiredFromBytes(pemBytes, now) {
		t.Error("certificate expires 1s after 24h threshold, should not be expired")
	}
}

func TestCertSoonExpiredFromBytesNilBlock(t *testing.T) {
	if !certSoonExpiredFromBytes([]byte("garbage"), time.Now()) {
		t.Error("nil PEM block should return true (soon expired)")
	}
}

func TestCertSoonExpiredFromBytesInvalidDER(t *testing.T) {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("bad")}
	pemBytes := pem.EncodeToMemory(block)
	if !certSoonExpiredFromBytes(pemBytes, time.Now()) {
		t.Error("invalid DER should return true (soon expired)")
	}
}

func TestBuildTargetAddrsBroadcast(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagBroadcast}
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 1, 10),
		Mask: net.CIDRMask(24, 32),
	}
	interfaces := map[*net.Interface][]*net.IPNet{iface: {ipNet}}
	ports := mapset.NewThreadUnsafeSet[uint16](1234)
	multicast := mapset.NewThreadUnsafeSet[netip.Addr]()

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	if len(mapping) != 1 {
		t.Fatalf("expected 1 source, got %d", len(mapping))
	}

	for source, targets := range mapping {
		if source.IP.String() != "192.168.1.10" {
			t.Errorf("source IP = %v, want 192.168.1.10", source.IP)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 broadcast target, got %d", len(targets))
		}
		if targets[0].IP.String() != "192.168.1.255" {
			t.Errorf("broadcast IP = %v, want 192.168.1.255", targets[0].IP)
		}
		if targets[0].Port != 1234 {
			t.Errorf("port = %d, want 1234", targets[0].Port)
		}
	}
}

func TestBuildTargetAddrsMulticast(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagMulticast}
	ipNet := &net.IPNet{
		IP:   net.IPv4(10, 0, 0, 1),
		Mask: net.CIDRMask(8, 32),
	}
	interfaces := map[*net.Interface][]*net.IPNet{iface: {ipNet}}
	ports := mapset.NewThreadUnsafeSet[uint16](5678)
	mcastAddr := netip.MustParseAddr("239.255.0.1")
	multicast := mapset.NewThreadUnsafeSet[netip.Addr](mcastAddr)

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	if len(mapping) != 1 {
		t.Fatalf("expected 1 source, got %d", len(mapping))
	}

	for _, targets := range mapping {
		if len(targets) != 1 {
			t.Fatalf("expected 1 multicast target, got %d", len(targets))
		}
		if targets[0].IP.String() != "239.255.0.1" {
			t.Errorf("multicast IP = %v, want 239.255.0.1", targets[0].IP)
		}
		if targets[0].Port != 5678 {
			t.Errorf("port = %d, want 5678", targets[0].Port)
		}
	}
}

func TestBuildTargetAddrsBothFlags(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagBroadcast | net.FlagMulticast}
	ipNet := &net.IPNet{
		IP:   net.IPv4(172, 16, 0, 5),
		Mask: net.CIDRMask(16, 32),
	}
	interfaces := map[*net.Interface][]*net.IPNet{iface: {ipNet}}
	ports := mapset.NewThreadUnsafeSet[uint16](80)
	mcastAddr := netip.MustParseAddr("224.0.0.1")
	multicast := mapset.NewThreadUnsafeSet[netip.Addr](mcastAddr)

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	for _, targets := range mapping {
		if len(targets) != 2 {
			t.Fatalf("expected 2 targets (broadcast + multicast), got %d", len(targets))
		}
	}
}

func TestBuildTargetAddrsNoFlags(t *testing.T) {
	iface := &net.Interface{Flags: 0}
	ipNet := &net.IPNet{
		IP:   net.IPv4(10, 0, 0, 1),
		Mask: net.CIDRMask(8, 32),
	}
	interfaces := map[*net.Interface][]*net.IPNet{iface: {ipNet}}
	ports := mapset.NewThreadUnsafeSet[uint16](80)
	multicast := mapset.NewThreadUnsafeSet[netip.Addr]()

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	for _, targets := range mapping {
		if len(targets) != 0 {
			t.Fatalf("expected 0 targets (no flags), got %d", len(targets))
		}
	}
}

func TestBuildTargetAddrsEmpty(t *testing.T) {
	interfaces := map[*net.Interface][]*net.IPNet{}
	ports := mapset.NewThreadUnsafeSet[uint16](80)
	multicast := mapset.NewThreadUnsafeSet[netip.Addr]()

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	if len(mapping) != 0 {
		t.Fatalf("expected empty mapping, got %d sources", len(mapping))
	}
}

func TestBuildTargetAddrsMultiplePorts(t *testing.T) {
	iface := &net.Interface{Flags: net.FlagBroadcast}
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 1, 1),
		Mask: net.CIDRMask(24, 32),
	}
	interfaces := map[*net.Interface][]*net.IPNet{iface: {ipNet}}
	ports := mapset.NewThreadUnsafeSet[uint16](80, 443, 8080)
	multicast := mapset.NewThreadUnsafeSet[netip.Addr]()

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	for _, targets := range mapping {
		if len(targets) != 3 {
			t.Fatalf("expected 3 broadcast targets, got %d", len(targets))
		}
	}
}

func TestBuildTargetAddrsMultipleInterfaces(t *testing.T) {
	iface1 := &net.Interface{Flags: net.FlagBroadcast}
	iface2 := &net.Interface{Flags: net.FlagBroadcast}
	ipNet1 := &net.IPNet{IP: net.IPv4(192, 168, 1, 1), Mask: net.CIDRMask(24, 32)}
	ipNet2 := &net.IPNet{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(8, 32)}
	interfaces := map[*net.Interface][]*net.IPNet{
		iface1: {ipNet1},
		iface2: {ipNet2},
	}
	ports := mapset.NewThreadUnsafeSet[uint16](1234)
	multicast := mapset.NewThreadUnsafeSet[netip.Addr]()

	mapping := buildTargetAddrs(interfaces, multicast, ports)
	if len(mapping) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(mapping))
	}
}
