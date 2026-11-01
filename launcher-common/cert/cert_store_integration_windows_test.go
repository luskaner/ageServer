//go:build windows

package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/luskaner/ageServer/common"
	"golang.org/x/sys/windows"
)

// TestWindowsCertStoreRoundTrip exercises the Windows certificate store round-trip.
// When AGESERVER_CERT_STORE_INTEGRATION=1 it touches the real CURRENT_USER "CA"
// store (integration). Otherwise it runs against an in-memory fake store so the
// test never skips and still guards the iterateContext ownership contract.
func TestWindowsCertStoreRoundTrip(t *testing.T) {
	if os.Getenv("AGESERVER_CERT_STORE_INTEGRATION") == "1" {
		testWindowsCertStoreRoundTripReal(t, "CA")
		return
	}
	testWindowsCertStoreRoundTripMock(t)
}

func testWindowsCertStoreRoundTripReal(t *testing.T, storeName string) {
	t.Helper()
	der, err := generateUniqueTestCert()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	if err = trustCertificatesInStore(true, storeName, []*x509.Certificate{cert}); err != nil {
		t.Fatalf("trust: %v", err)
	}

	found, err := enumCertificatesInStore(true, storeName)
	if err != nil {
		t.Fatalf("enum after trust: %v", err)
	}
	if !containsCert(found, cert) {
		t.Fatal("trusted certificate not found in store")
	}

	removed, err := untrustCertificatesFromStore(true, storeName)
	if err != nil {
		t.Fatalf("untrust: %v", err)
	}
	if !containsCert(removed, cert) {
		t.Fatal("untrust did not report removing the certificate")
	}

	found, err = enumCertificatesInStore(true, storeName)
	if err != nil {
		t.Fatalf("enum after untrust: %v", err)
	}
	if containsCert(found, cert) {
		t.Fatal("certificate still present after untrust")
	}
}

func generateUniqueTestCert() ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "ageServer store integration test",
			Organization: []string{common.CertSubjectOrganization},
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	return x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
}

func containsCert(certs []*x509.Certificate, want *x509.Certificate) bool {
	for _, c := range certs {
		if c.Equal(want) {
			return true
		}
	}
	return false
}

// Mocked in-memory store tests – always runs, no real Windows store touched.
// It validates the same round-trip and the iterateContext ownership contract.

func testWindowsCertStoreRoundTripMock(t *testing.T) {
	t.Helper()
	// Install fake Windows API
	restore := installFakeStore(t)
	defer restore()

	const storeName = "CA"

	der, err := generateUniqueTestCert()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	// Trust
	if err = trustCertificatesInStore(true, storeName, []*x509.Certificate{cert}); err != nil {
		t.Fatalf("trust (mock): %v", err)
	}
	// Duplicate trust must fail (CERT_STORE_ADD_NEW)
	if err = trustCertificatesInStore(true, storeName, []*x509.Certificate{cert}); err == nil {
		t.Fatal("duplicate trust should fail")
	}

	found, err := enumCertificatesInStore(true, storeName)
	if err != nil {
		t.Fatalf("enum after trust (mock): %v", err)
	}
	if !containsCert(found, cert) {
		t.Fatal("trusted certificate not found in mock store")
	}

	removed, err := untrustCertificatesFromStore(true, storeName)
	if err != nil {
		t.Fatalf("untrust (mock): %v", err)
	}
	if !containsCert(removed, cert) {
		t.Fatal("untrust did not report removing the certificate (mock)")
	}

	found, err = enumCertificatesInStore(true, storeName)
	if err != nil {
		t.Fatalf("enum after untrust (mock): %v", err)
	}
	if containsCert(found, cert) {
		t.Fatal("certificate still present after untrust (mock)")
	}
	// Untrust again on empty store should return empty without error (iterateContext clears err when certs>0 logic)
	// Here empty, so should return empty slice, no error
	removed, err = untrustCertificatesFromStore(true, storeName)
	if err != nil {
		t.Fatalf("second untrust (mock) should not error: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("second untrust should return empty, got %d", len(removed))
	}
}

func TestIterateContextOwnership(t *testing.T) {
	restore := installFakeStore(t)
	defer restore()

	// Seed store with two certs
	certs := make([]*x509.Certificate, 2)
	for i := range certs {
		der, err := generateUniqueTestCert()
		if err != nil {
			t.Fatal(err)
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		certs[i] = c
	}
	if err := trustCertificatesInStore(true, "CA", certs); err != nil {
		t.Fatalf("trust: %v", err)
	}

	// Enum uses actionTakesOwnership=false – should not leak contexts and not double-free
	found, err := enumCertificatesInStore(true, "CA")
	if err != nil {
		t.Fatalf("enum: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2, got %d", len(found))
	}
	if len(fakeContextBytes) != 0 {
		t.Errorf("enum leaked %d contexts, expected 0 (ownership=false should free via Enum)", len(fakeContextBytes))
	}
	// Verify cursors cleaned
	if len(fakeEnumCursor) != 0 {
		t.Errorf("enum cursor not cleaned")
	}

	// Untrust uses ownership=true – Delete frees each context, iterateContext must not double-free
	removed, err := untrustCertificatesFromStore(true, "CA")
	if err != nil {
		t.Fatalf("untrust: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
	if len(fakeContextBytes) != 0 {
		t.Errorf("untrust leaked %d contexts", len(fakeContextBytes))
	}
	if len(fakeFindCursor) != 0 {
		t.Errorf("find cursor not cleaned")
	}
	// Ensure store is empty
	if len(fakeStores) > 0 {
		for _, list := range fakeStores {
			if len(list) != 0 {
				t.Errorf("store not empty after untrust, %d certs remain", len(list))
			}
		}
	}
}

func TestIterateContextOpenFailure(t *testing.T) {
	origOpen := certOpenStoreFn
	certOpenStoreFn = func(provider uintptr, encoding uint32, hCryptProv uintptr, flags uint32, para uintptr) (windows.Handle, error) {
		return 0, errors.New("open failed")
	}
	defer func() { certOpenStoreFn = origOpen }()

	_, err := enumCertificatesInStore(true, "CA")
	if err == nil || err.Error() != "open failed" {
		t.Fatalf("expected open failure, got %v", err)
	}
}

// --- Fake store infrastructure ---

var (
	fakeStores                      = map[windows.Handle][]*x509.Certificate{}
	fakeHandleByName                = map[string]windows.Handle{}
	fakeNextHandle   windows.Handle = 100
	fakeContextBytes                = map[*windows.CertContext][]byte{}
	fakeEnumCursor                  = map[windows.Handle]int{}
	fakeFindCursor                  = map[windows.Handle]int{}
)

func installFakeStore(t *testing.T) func() {
	t.Helper()
	// Save originals
	origOpen := certOpenStoreFn
	origClose := certCloseStoreFn
	origCreate := certCreateCertificateContextFn
	origAdd := certAddCertificateContextToStoreFn
	origEnum := certEnumCertificatesInStoreFn
	origFind := certFindCertificateInStoreFn
	origDelete := certDeleteCertificateFromStoreFn
	origFree := certFreeCertificateContextFn

	// Reset fake state
	fakeStores = map[windows.Handle][]*x509.Certificate{}
	fakeHandleByName = map[string]windows.Handle{}
	fakeNextHandle = 100
	fakeContextBytes = map[*windows.CertContext][]byte{}
	fakeEnumCursor = map[windows.Handle]int{}
	fakeFindCursor = map[windows.Handle]int{}

	certOpenStoreFn = func(provider uintptr, encoding uint32, hCryptProv uintptr, flags uint32, para uintptr) (windows.Handle, error) {
		// Single in-memory store for test – ignore para/name, always return same handle
		if len(fakeStores) > 0 {
			for h := range fakeStores {
				return h, nil
			}
		}
		h := fakeNextHandle
		fakeNextHandle++
		fakeStores[h] = []*x509.Certificate{}
		return h, nil
	}
	certCloseStoreFn = func(h windows.Handle, flags uint32) error { return nil }
	certCreateCertificateContextFn = func(encoding uint32, certEncoded *byte, encodedLen uint32) (*windows.CertContext, error) {
		if certEncoded == nil && encodedLen != 0 {
			return nil, errors.New("nil cert")
		}
		b := make([]byte, encodedLen)
		if encodedLen > 0 {
			src := unsafe.Slice(certEncoded, encodedLen)
			copy(b, src)
		}
		if _, err := x509.ParseCertificate(b); err != nil {
			return nil, err
		}
		ctx := &windows.CertContext{
			EncodedCert: &b[0],
			Length:      encodedLen,
		}
		// Keep alive
		fakeContextBytes[ctx] = b
		return ctx, nil
	}
	certAddCertificateContextToStoreFn = func(store windows.Handle, ctx *windows.CertContext, disposition uint32, out **windows.CertContext) error {
		b, ok := fakeContextBytes[ctx]
		if !ok {
			// fallback read directly
			b = unsafe.Slice(ctx.EncodedCert, ctx.Length)
		}
		cert, err := x509.ParseCertificate(b)
		if err != nil {
			return err
		}
		list := fakeStores[store]
		for _, c := range list {
			if c.Equal(cert) {
				return errors.New("cert already exists")
			}
		}
		fakeStores[store] = append(list, cert)
		return nil
	}
	certEnumCertificatesInStoreFn = func(store windows.Handle, prev *windows.CertContext) (*windows.CertContext, error) {
		// Windows frees prev when given
		if prev != nil {
			delete(fakeContextBytes, prev)
		}
		list := fakeStores[store]
		if len(list) == 0 {
			return nil, nil
		}
		// Use per-store cursor – prev is only used to know if this is continuation
		if prev == nil {
			// first call
			cert := list[0]
			b := make([]byte, len(cert.Raw))
			copy(b, cert.Raw)
			ctx := &windows.CertContext{EncodedCert: &b[0], Length: uint32(len(b))}
			fakeContextBytes[ctx] = b
			// Store cursor
			fakeEnumCursor[store] = 1
			return ctx, nil
		}
		// subsequent calls: use cursor
		cursor := fakeEnumCursor[store]
		if cursor >= len(list) {
			delete(fakeEnumCursor, store)
			return nil, nil
		}
		cert := list[cursor]
		b := make([]byte, len(cert.Raw))
		copy(b, cert.Raw)
		ctx := &windows.CertContext{EncodedCert: &b[0], Length: uint32(len(b))}
		fakeContextBytes[ctx] = b
		fakeEnumCursor[store] = cursor + 1
		return ctx, nil
	}
	certFindCertificateInStoreFn = func(store windows.Handle, encoding uint32, flags uint32, findType uint32, findPara unsafe.Pointer, prev *windows.CertContext) (*windows.CertContext, error) {
		if prev != nil {
			delete(fakeContextBytes, prev)
		}
		var search string
		if findPara != nil {
			search = windows.UTF16PtrToString((*uint16)(findPara))
		}
		list := fakeStores[store]
		start := 0
		if prev == nil {
			start = 0
		} else {
			start = fakeFindCursor[store]
		}
		for i := start; i < len(list); i++ {
			c := list[i]
			for _, org := range c.Subject.Organization {
				if org == search {
					b := make([]byte, len(c.Raw))
					copy(b, c.Raw)
					ctx := &windows.CertContext{EncodedCert: &b[0], Length: uint32(len(b))}
					fakeContextBytes[ctx] = b
					fakeFindCursor[store] = i + 1
					return ctx, nil
				}
			}
		}
		delete(fakeFindCursor, store)
		return nil, nil
	}
	certDeleteCertificateFromStoreFn = func(ctx *windows.CertContext) error {
		b, ok := fakeContextBytes[ctx]
		if !ok && ctx.EncodedCert != nil {
			b = unsafe.Slice(ctx.EncodedCert, ctx.Length)
		}
		if b == nil {
			return errors.New("unknown context")
		}
		cert, err := x509.ParseCertificate(b)
		if err != nil {
			return err
		}
		for h, list := range fakeStores {
			for i, c := range list {
				if c.Equal(cert) {
					fakeStores[h] = append(list[:i], list[i+1:]...)
					delete(fakeContextBytes, ctx)
					return nil
				}
			}
		}
		return errors.New("not found")
	}
	certFreeCertificateContextFn = func(ctx *windows.CertContext) error {
		delete(fakeContextBytes, ctx)
		return nil
	}

	return func() {
		certOpenStoreFn = origOpen
		certCloseStoreFn = origClose
		certCreateCertificateContextFn = origCreate
		certAddCertificateContextToStoreFn = origAdd
		certEnumCertificatesInStoreFn = origEnum
		certFindCertificateInStoreFn = origFind
		certDeleteCertificateFromStoreFn = origDelete
		certFreeCertificateContextFn = origFree
		// Clear fake state
		fakeStores = map[windows.Handle][]*x509.Certificate{}
		fakeHandleByName = map[string]windows.Handle{}
		fakeContextBytes = map[*windows.CertContext][]byte{}
	}
}
