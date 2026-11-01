package logger

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"github.com/luskaner/ageServer/common"
)

func TestFilterMatchingCertsByCN(t *testing.T) {
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: common.Name},
	}
	matched := filterMatchingCerts([]*x509.Certificate{cert}, []string{"other.com"})
	if len(matched) != 1 {
		t.Fatalf("expected 1 match by CN, got %d", len(matched))
	}
}

func TestFilterMatchingCertsByDNSNames(t *testing.T) {
	cert := &x509.Certificate{
		Subject:  pkix.Name{CommonName: "irrelevant"},
		DNSNames: []string{"*.worldsedgelink.com"},
	}
	matched := filterMatchingCerts([]*x509.Certificate{cert}, []string{"AoE2-DE.worldsedgelink.com"})
	if len(matched) != 1 {
		t.Fatalf("expected 1 match by DNS SAN, got %d", len(matched))
	}
}

func TestFilterMatchingCertsNoMatch(t *testing.T) {
	cert := &x509.Certificate{
		Subject:  pkix.Name{CommonName: "other.com"},
		DNSNames: []string{"other.com"},
	}
	matched := filterMatchingCerts([]*x509.Certificate{cert}, []string{"example.com"})
	if len(matched) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matched))
	}
}

func TestFilterMatchingCertsEmpty(t *testing.T) {
	matched := filterMatchingCerts(nil, []string{"example.com"})
	if len(matched) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matched))
	}
}

func TestFilterMatchingCertsFallbackCNMatch(t *testing.T) {
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: "example.com"},
	}
	matched := filterMatchingCerts([]*x509.Certificate{cert}, []string{"example.com"})
	if len(matched) != 1 {
		t.Fatalf("expected 1 fallback CN match, got %d", len(matched))
	}
}

func TestWriteCertificateInfoNoCerts(t *testing.T) {
	err := writeCertificateInfo(nil, []string{"example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteCertificateInfoWithMatching(t *testing.T) {
	cert := &x509.Certificate{
		Subject:  pkix.Name{CommonName: "myhost"},
		DNSNames: []string{"myhost", "other.com"},
	}
	err := writeCertificateInfo([]*x509.Certificate{cert}, []string{"myhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteCertificateInfoWithNoDNS(t *testing.T) {
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: common.Name},
	}
	err := writeCertificateInfo([]*x509.Certificate{cert}, []string{"example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteLogSuccess(t *testing.T) {
	err := writeLog("AoE2", "test", func(gameId string) error {
		if gameId != "AoE2" {
			t.Errorf("expected gameId AoE2, got %s", gameId)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteLogError(t *testing.T) {
	err := writeLog("AoE2", "test", func(gameId string) error {
		return &mockError{"boom"}
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
