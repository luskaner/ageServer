package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/executor/exec"
)

// A session trusts two certificates through one store file (the API server's
// and, afterwards, the root server's), so the second trust call has to keep
// the first one's certificate and the reversal has to return both.
func TestLinuxStoreHoldsEveryCertificateAcrossPasses(t *testing.T) {
	storeDir := t.TempDir()
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		t.Fatalf("failed to create the store dir: %v", err)
	}
	origPaths := certStorePaths
	certStorePaths = []string{storeDir}
	t.Cleanup(func() { certStorePaths = origPaths })
	// The store update binary needs root and pkexec, which a CI runner does not
	// have: what this test covers is the bundle, not the system's rebuild.
	origFlush := flushCertsFn
	flushCertsFn = func() *exec.Result { return &exec.Result{} }
	t.Cleanup(func() { flushCertsFn = origFlush })

	certA := generatedCert(t, "api root", 1)
	certB := generatedCert(t, "server root", 2)

	if err := TrustCertificates(false, []*x509.Certificate{certA}); err != nil {
		t.Fatalf("the API pass's trust failed: %v", err)
	}
	if err := TrustCertificates(false, []*x509.Certificate{certB}); err != nil {
		t.Fatalf("the server pass's trust failed: %v", err)
	}

	path := filepath.Join(storeDir, "ageServer.crt")
	held, err := readStoreCertificates(path)
	if err != nil {
		t.Fatalf("the store file is not readable: %v", err)
	}
	if len(held) != 2 {
		t.Fatalf("the store holds %d certificates, want 2 (API pass's certificate must survive the server pass)", len(held))
	}
	if !strings.Contains(certificateNames(held), "api root,server root") {
		t.Fatalf("the store holds [%s], want both passes' certificates in order", certificateNames(held))
	}

	// A repeated pass must not grow the file.
	if err = TrustCertificates(false, []*x509.Certificate{certB}); err != nil {
		t.Fatalf("the repeated trust failed: %v", err)
	}
	if held, err = readStoreCertificates(path); err != nil || len(held) != 2 {
		t.Fatalf("a repeated pass changed the store: %d certs, err=%v", len(held), err)
	}

	removed, err := UntrustCertificates(false)
	if err != nil {
		t.Fatalf("the untrust failed: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("the untrust returned %d certificates, want 2", len(removed))
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("the store file survived the untrust: %v", statErr)
	}
}

func certificateNames(certs []*x509.Certificate) string {
	names := make([]string, 0, len(certs))
	for _, cert := range certs {
		names = append(names, cert.Subject.CommonName)
	}
	return strings.Join(names, ",")
}

func generatedCert(t *testing.T, commonName string, serial int64) *x509.Certificate {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate the key: %v", err)
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create the certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse the certificate: %v", err)
	}
	return cert
}
