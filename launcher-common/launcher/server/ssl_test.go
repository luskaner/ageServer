package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func generateTestCertAndKey(t *testing.T) (certPem []byte, keyPem []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPem = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	return
}

func startTLSServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	certPem, keyPem := generateTestCertAndKey(t)
	pair, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler:   handler,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", srv.TLSConfig)
	if err != nil {
		t.Skipf("cannot bind: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = ln.Close() })
	return "127.0.0.1:" + strconv.Itoa(port)
}

func TestReadCACertificateFromServerSuccess(t *testing.T) {
	certPem, _ := generateTestCertAndKey(t)

	host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(certPem)
	}))

	result := ReadCACertificateFromServer(host)
	if result == nil {
		t.Fatal("expected valid certificate")
	}
}

// Regression: when the server returned non-200, resp.Body was never closed
// because the defer was registered only after io.ReadAll succeeded.
func TestReadCACertificateFromServerNon200CleansUp(t *testing.T) {
	host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	result := ReadCACertificateFromServer(host)
	if result != nil {
		t.Fatal("non-200 must return nil")
	}
}

// The client is pointed at this file so it verifies catalogues against the key
// that signed them, so the bytes must be the exact DER the server serves. A
// re-encoded equivalent would still be a valid key but no longer the same file
// the server's own check compares against.
func TestReadSigningKeyFromServerReturnsServedBytes(t *testing.T) {
	der := testSigningKeyDER(t)

	host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sigkey.der" {
			t.Errorf("path = %q, want /sigkey.der", r.URL.Path)
		}
		_, _ = w.Write(der)
	}))

	got := ReadSigningKeyFromServer(host)
	if !bytes.Equal(got, der) {
		t.Fatalf("key = %x, want the served key %x", got, der)
	}
}

// testSigningKeyDER returns an RSA-2048 SPKI, the only shape the client's
// in-place substitution accepts: it is 294 bytes, the length it overwrites.
func testSigningKeyDER(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(der) != signingKeySize {
		t.Fatalf("test key is %d bytes, want %d", len(der), signingKeySize)
	}
	return der
}

// An error page served with a 200, a proxy, or a stale route must not be handed
// to the client as if it were a key.
func TestReadSigningKeyFromServerRejectsNonKey(t *testing.T) {
	host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not a key</html>"))
	}))

	if got := ReadSigningKeyFromServer(host); got != nil {
		t.Fatalf("key = %q, want nil for a body that is not a public key", got)
	}
}

// A key of another type or size is not one the client can substitute: the DLL
// writes it over exactly 294 bytes and fails otherwise, without telling the user.
func TestReadSigningKeyFromServerRejectsAKeyOfTheWrongShape(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKIXPublicKey(&ecKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	small, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	smallDER, err := x509.MarshalPKIXPublicKey(&small.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, der := range map[string][]byte{"ecdsa": ecDER, "rsa-1024": smallDER} {
		t.Run(name, func(t *testing.T) {
			host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(der)
			}))
			if got := ReadSigningKeyFromServer(host); got != nil {
				t.Fatalf("accepted a %d-byte key the client cannot use", len(got))
			}
		})
	}
}

func TestReadSigningKeyFromServerNon200IsNil(t *testing.T) {
	host := startTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	if got := ReadSigningKeyFromServer(host); got != nil {
		t.Fatalf("key = %q, want nil for a missing key", got)
	}
}
