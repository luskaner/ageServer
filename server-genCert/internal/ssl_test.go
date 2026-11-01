package internal

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
)

func generateIntoTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if !GenerateCertificatePairs(dir) {
		t.Fatal("GenerateCertificatePairs reported failure")
	}
	return dir
}

func readCertificate(t *testing.T, dir string, name string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("%s: no PEM block", name)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	return cert
}

// Regression: certificates used to be valid starting from exactly time.Now(),
// so any client with a slightly ahead-set clock rejected them as
// "not yet valid" right after generation.
func TestGeneratedCertificatesAreBackdated(t *testing.T) {
	dir := generateIntoTemp(t)
	skewLimit := time.Now().Add(-30 * time.Second)

	for _, name := range []string{common.CACert, common.Cert, common.SelfSignedCert} {
		cert := readCertificate(t, dir, name)
		if cert.NotBefore.After(skewLimit) {
			t.Errorf("%s NotBefore = %v, want backdated at least 30s", name, cert.NotBefore)
		}
		if !cert.NotAfter.After(time.Now()) {
			t.Errorf("%s already expired", name)
		}
	}
}

func TestGeneratedChainVerifiesAgainstCA(t *testing.T) {
	dir := generateIntoTemp(t)
	ca := readCertificate(t, dir, common.CACert)
	leaf := readCertificate(t, dir, common.Cert)

	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("leaf does not verify against CA: %v", err)
	}
}

// Regression: private keys were created world-readable (0666 & umask).
func TestGeneratedPrivateKeysAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not apply on Windows")
	}
	dir := generateIntoTemp(t)
	for _, name := range []string{common.Key, common.SelfSignedKey} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if perm := fi.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s permissions = %#o, group/other bits must be unset", name, perm)
		}
	}
}

func TestGeneratedKeyPairsLoadAsTLSKeyPair(t *testing.T) {
	dir := generateIntoTemp(t)
	for _, pair := range [][2]string{
		{common.Cert, common.Key},
		{common.SelfSignedCert, common.SelfSignedKey},
	} {
		certPEM, err := os.ReadFile(filepath.Join(dir, pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		keyPEM, err := os.ReadFile(filepath.Join(dir, pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tls.X509KeyPair(certPEM, keyPEM); err != nil {
			t.Errorf("%s/%s: %v", pair[0], pair[1], err)
		}
	}
}

func TestGetTemplateSelfSigned(t *testing.T) {
	tmpl := getTemplate("selfsigned")
	if !tmpl.IsCA {
		t.Error("selfsigned should be CA")
	}
	if !tmpl.MaxPathLenZero {
		t.Error("selfsigned should have MaxPathLenZero")
	}
	if tmpl.Subject.CommonName != common.Name+" Self-signed" {
		t.Errorf("CN = %q", tmpl.Subject.CommonName)
	}
}

func TestGetTemplateCA(t *testing.T) {
	tmpl := getTemplate("ca")
	if !tmpl.IsCA {
		t.Error("ca should be CA")
	}
	if tmpl.Subject.CommonName != common.Name+" CA" {
		t.Errorf("CN = %q", tmpl.Subject.CommonName)
	}
	if tmpl.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("ca should have KeyUsageCertSign")
	}
}

func TestGetTemplateNormal(t *testing.T) {
	tmpl := getTemplate("normal")
	if tmpl.IsCA {
		t.Error("normal should not be CA")
	}
	if len(tmpl.DNSNames) == 0 {
		t.Error("normal should have DNS names")
	}
}

func TestGenerateSelfSignedCertificateReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	if generateSelfSignedCertificate(dir) {
		t.Error("should fail in read-only directory")
	}
}

func TestGenerateCertificatePairsReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	ok, _ := generateCertificatePairs(dir, "cert.pem", "key.pem", nil, nil)
	if ok {
		t.Error("should fail in read-only directory")
	}
}

func TestGenerateCertificatePairsCAKeyBuffer(t *testing.T) {
	dir := t.TempDir()
	ok, caKey := generateCertificatePairs(dir, "ca.pem", "", nil, nil)
	if !ok {
		t.Fatal("CA generation failed")
	}
	if caKey == nil || caKey.Len() == 0 {
		t.Error("CA key buffer should be non-empty")
	}
	if _, err := os.Stat(filepath.Join(dir, "ca.pem")); err != nil {
		t.Errorf("CA cert file should exist: %v", err)
	}
}

func TestGenerateCertificatePairsNormalWithParent(t *testing.T) {
	dir := t.TempDir()
	// Generate CA first
	ok, caKey := generateCertificatePairs(dir, "ca.pem", "", nil, nil)
	if !ok {
		t.Fatal("CA generation failed")
	}
	caCertPEM, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		t.Fatal("no PEM block in CA cert")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// caKey is PEM-encoded, need to decode it
	caKeyBlock, _ := pem.Decode(caKey.Bytes())
	caKeyParsed, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	ok, _ = generateCertificatePairs(dir, "server.pem", "server.key", caCert, caKeyParsed)
	if !ok {
		t.Fatal("server cert generation failed")
	}
}

func TestGenerateCertificatePairsCertCreateFails(t *testing.T) {
	// Use a non-existent path to trigger os.Create failure
	ok, _ := generateCertificatePairs("/nonexistent/path/that/does/not/exist", "cert.pem", "key.pem", nil, nil)
	if ok {
		t.Error("should fail with non-existent path")
	}
}

func TestGenerateCertificatePairsKeyCreateFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	ok, _ := generateCertificatePairs(dir, "cert.pem", "key.pem", nil, nil)
	if ok {
		t.Error("should fail when key file cannot be created")
	}
}

func TestGenerateCertificatePairsCABackupOnServerFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	dir := t.TempDir()
	// Generate CA successfully
	if ok, _ := generateCertificatePairs(dir, "ca.pem", "", nil, nil); !ok {
		t.Fatal("CA generation failed")
	}
	// Server cert generation will also try to generate self-signed after that,
	// so make dir read-only after CA generation to fail on server cert creation.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	ok := GenerateCertificatePairs(dir)
	if ok {
		t.Error("should fail when server cert generation fails")
	}
}

func TestGenerateCertificatePairsSelfSignedFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	dir := t.TempDir()
	ok, caKey := generateCertificatePairs(dir, "ca.pem", "", nil, nil)
	if !ok {
		t.Fatal("CA generation failed")
	}
	caCertPEM, _ := os.ReadFile(filepath.Join(dir, "ca.pem"))
	block, _ := pem.Decode(caCertPEM)
	caCert, _ := x509.ParseCertificate(block.Bytes)
	caKeyBlock, _ := pem.Decode(caKey.Bytes())
	caKeyParsed, _ := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	ok, _ = generateCertificatePairs(dir, "cert.pem", "key.pem", caCert, caKeyParsed)
	if !ok {
		t.Fatal("server cert generation failed")
	}
	// Make dir read-only so self-signed generation fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	ok = GenerateCertificatePairs(dir)
	if ok {
		t.Error("should fail when self-signed generation fails")
	}
}
