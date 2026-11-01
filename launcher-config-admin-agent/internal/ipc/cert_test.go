package ipc

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
)

// "not_valid" contains underscores which are invalid in IDNA/DNS.
const invalidIDNACN = "not_valid"

func validGameCert(gameId string) *x509.Certificate {
	domains := make([]string, len(common.SelfSignedCertDomains))
	copy(domains, common.SelfSignedCertDomains)
	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: invalidIDNACN,
		},
		IsCA:           true,
		MaxPathLenZero: true,
		DNSNames:       domains,
		KeyUsage:       x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		NotBefore:      time.Now(),
		NotAfter:       time.Now().Add(time.Hour),
	}
}

func validNonGameCert() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: invalidIDNACN,
		},
		IsCA:        true,
		KeyUsage:    x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour),
	}
}

func TestCheckCertValidityNilCert(t *testing.T) {
	if checkCertificateValidity(nil, game.AoE2) {
		t.Fatal("nil cert should be rejected")
	}
}

func TestCheckCertValidityWildcardCN(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.Subject.CommonName = "*.example.com"
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("wildcard CN should be rejected")
	}
}

func TestCheckCertValidityIDNACN(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.Subject.CommonName = "münchen" // valid IDNA
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("IDNA-valid CN should be rejected")
	}
}

func TestCheckCertValidityIPCN(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.Subject.CommonName = "192.168.1.1"
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("IP CN should be rejected")
	}
}

func TestCheckCertValidityNotCA(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.IsCA = false
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("non-CA cert should be rejected")
	}
}

func TestCheckCertValidityGameCertMissingMaxPathLenZero(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.MaxPathLenZero = false
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("game cert without MaxPathLenZero should be rejected")
	}
}

func TestCheckCertValidityGameCertWrongDNSNames(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.DNSNames = []string{"wrong.example.com"}
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("game cert with wrong DNS names should be rejected")
	}
}

func TestCheckCertValidityGameCertWrongKeyUsage(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.KeyUsage = x509.KeyUsageCertSign // missing KeyEncipherment + DigitalSignature
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("game cert with wrong KeyUsage should be rejected")
	}
}

func TestCheckCertValidityGameCertWrongExtKeyUsage(t *testing.T) {
	cert := validGameCert(game.AoE2)
	cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth} // missing ClientAuth
	if checkCertificateValidity(cert, game.AoE2) {
		t.Fatal("game cert with wrong ExtKeyUsage should be rejected")
	}
}

func TestCheckCertValidityNonGameCertValid(t *testing.T) {
	cert := validNonGameCert()
	if !checkCertificateValidity(cert, game.AoE4) {
		t.Fatal("valid non-game cert should be accepted for AoE4")
	}
}

func TestCheckCertValidityNonGameCertWrongKeyUsage(t *testing.T) {
	cert := validNonGameCert()
	cert.KeyUsage = x509.KeyUsageDigitalSignature
	if checkCertificateValidity(cert, game.AoE4) {
		t.Fatal("non-game cert with wrong KeyUsage should be rejected")
	}
}

func TestCheckCertValidityGameCertAoE1(t *testing.T) {
	cert := validGameCert(game.AoE1)
	if !checkCertificateValidity(cert, game.AoE1) {
		t.Fatal("valid game cert should be accepted for AoE1")
	}
}

func TestCheckCertValidityGameCertAoE3(t *testing.T) {
	cert := validGameCert(game.AoE3)
	if !checkCertificateValidity(cert, game.AoE3) {
		t.Fatal("valid game cert should be accepted for AoE3")
	}
}

func TestCheckCertValidityAoE4NoSelfSignedCert(t *testing.T) {
	// AoE4 does NOT use self-signed certs (SelfSignedCertGame returns false)
	// so the DNS names / MaxPathLenZero checks are skipped
	cert := validNonGameCert()
	if !checkCertificateValidity(cert, game.AoE4) {
		t.Fatal("valid non-game cert should be accepted for AoE4")
	}
}

func TestCheckCertValidityAoMNoSelfSignedCert(t *testing.T) {
	cert := validNonGameCert()
	if !checkCertificateValidity(cert, game.AoM) {
		t.Fatal("valid non-game cert should be accepted for AoM")
	}
}
