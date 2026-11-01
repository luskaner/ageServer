package models

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"hash/crc32"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// A catalogue body shaped like responses/<game>/itemDefinitions.json.
const testBody = `{"result":0,"itemCategories":[],"itemDefinitions":[],"dataSignature":""}`

// useTestKey installs a throwaway key and pins the once so the loader does not
// run.
func useTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signingKeyOnce.Do(func() {})
	signingKey = key
	signingKeyOK = true
	signingKeyErr = nil
	return key
}

// TestStampSignatureBlockLayout is the important one: it reproduces what the
// client does on read and asserts the recovered block matches the scheme in
// SERVER_SIGNING_SPEC.md exactly - 0x00 0x01, maximum-length FF padding, the
// bare SHA-512 digest with no ASN.1 DigestInfo.
func TestStampSignatureBlockLayout(t *testing.T) {
	key := useTestKey(t)
	body := []byte(testBody)

	stamped, sig, err := StampSignature(body)
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if len(sig) != signatureLength {
		t.Fatalf("signature length = %d, want %d", len(sig), signatureLength)
	}
	if got := SignatureOf(stamped); got != sig {
		t.Fatalf("stamped body carries %q, want %q", got, sig)
	}

	// Client side: locate the signature with strstr and splice it out, then crc.
	// The body carries the base64 text, so that is what strstr searches for.
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(raw) != 256 {
		t.Fatalf("decoded signature = %d bytes, want 256", len(raw))
	}
	pos := bytes.Index(stamped, []byte(sig))
	if pos < 0 {
		t.Fatal("client strstr would not find the signature in the body")
	}
	clientBody := append(append([]byte{}, stamped[:pos]...), stamped[pos+len(sig):]...)

	stripped := StripSignature(body)
	if !bytes.Equal(clientBody, stripped) {
		t.Fatalf("client splice does not reproduce the stripped body\n got: %s\nwant: %s",
			clientBody, stripped)
	}

	// Public-key operation, exactly as the client's rsa_public_op does.
	em := new(big.Int).Exp(new(big.Int).SetBytes(raw), big.NewInt(int64(key.E)), key.N)
	block := em.FillBytes(make([]byte, 256))

	if block[0] != 0x00 || block[1] != 0x01 {
		t.Fatalf("block[0:2] = %02X %02X, want 00 01", block[0], block[1])
	}
	if block[191] != 0x00 {
		t.Fatalf("block[191] = %02X, want 00", block[191])
	}
	padding := 0
	for i := 2; i < 191; i++ {
		if block[i] != 0xFF {
			t.Fatalf("block[%d] = %02X, want FF", i, block[i])
		}
		padding++
	}
	if padding != 189 {
		t.Fatalf("padding = %d bytes, want 189", padding)
	}

	// The tail must be the bare digest of the decimal crc - no DigestInfo.
	crc := crc32.ChecksumIEEE(stripped)
	digest := sha512.Sum512([]byte(strconv.FormatUint(uint64(crc), 10)))
	if !bytes.Equal(block[192:], digest[:]) {
		t.Fatalf("tail is not SHA-512(%q)\n got: %x\nwant: %x",
			strconv.FormatUint(uint64(crc), 10), block[192:], digest)
	}
	if bytes.HasPrefix(block[192:], []byte{0x30, 0x51, 0x30, 0x0D, 0x06, 0x09}) {
		t.Fatal("tail carries an ASN.1 DigestInfo prefix; RelicLink signs the bare digest")
	}
}

// TestStampSignatureIsIdempotent: the crc is taken over the stripped form, so
// re-stamping a stamped body must not drift.
func TestStampSignatureIsIdempotent(t *testing.T) {
	useTestKey(t)
	body := []byte(testBody)

	once, sigOnce, err := StampSignature(body)
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	twice, sigTwice, err := StampSignature(once)
	if err != nil {
		t.Fatalf("re-stamp: %v", err)
	}

	if sigOnce != sigTwice {
		t.Fatalf("signature changed on re-stamp:\n %s\n %s", sigOnce, sigTwice)
	}
	if !bytes.Equal(once, twice) {
		t.Fatal("body changed on re-stamp")
	}
}

// Distinct bodies must yield distinct signatures, because the client uses
// dataSignature as an ETag in the ?signature= query parameter.
func TestDistinctBodiesDistinctSignatures(t *testing.T) {
	useTestKey(t)
	a, sigA, err := StampSignature([]byte(testBody))
	if err != nil {
		t.Fatalf("stamp a: %v", err)
	}
	b, sigB, err := StampSignature([]byte(`{"result":0,"itemCategories":[1],"itemDefinitions":[],"dataSignature":""}`))
	if err != nil {
		t.Fatalf("stamp b: %v", err)
	}

	if bytes.Equal(a, b) {
		t.Fatal("bodies differ but produced equal output")
	}
	if sigA == sigB {
		t.Fatal("bodies differ but produced the same signature")
	}
}

// With no key there is nothing to sign with, and the client only accepts a
// signature it can verify: that has to surface as an error at the call site,
// not as a well-formed placeholder that fails later, on the client.
func TestStampSignatureWithoutKey(t *testing.T) {
	signingKeyOnce.Do(func() {})
	signingKey = nil
	signingKeyOK = false
	signingKeyErr = nil
	t.Cleanup(func() {
		signingKeyOnce = sync.Once{}
		signingKey = nil
		signingKeyOK = false
		signingKeyErr = nil
	})

	stamped, sig, err := StampSignature([]byte(testBody))
	if err == nil {
		t.Fatal("stamping without a key should fail")
	}
	if stamped != nil || sig != "" {
		t.Fatal("a failed stamp must not return a body or a signature")
	}
	if SigningKeyConfigured() {
		t.Fatal("SigningKeyConfigured should be false with no key")
	}
}

// A payload signed outside a catalogue body must follow the very same scheme,
// or a client verifying it the way it verifies dataSignature will reject it.
func TestSignPayloadUsesTheSameScheme(t *testing.T) {
	key := useTestKey(t)
	payload := []byte(`[1,2,3]`)

	sig, err := SignPayload(payload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	if len(sig) != signatureLength {
		t.Fatalf("signature length = %d, want %d", len(sig), signatureLength)
	}

	// Client side: recover the block with the public key, then check the tail
	// is the bare SHA-512 of the decimal crc32, with no DigestInfo prefix.
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	em := new(big.Int).Exp(new(big.Int).SetBytes(raw), big.NewInt(int64(key.E)), key.N)
	block := em.FillBytes(make([]byte, 256))
	if block[0] != 0x00 || block[1] != 0x01 || block[191] != 0x00 {
		t.Fatalf("block header = %02X %02X ... %02X, want 00 01 ... 00",
			block[0], block[1], block[191])
	}
	crc := crc32.ChecksumIEEE(payload)
	digest := sha512.Sum512([]byte(strconv.FormatUint(uint64(crc), 10)))
	if !bytes.Equal(block[192:], digest[:]) {
		t.Fatalf("tail is not SHA-512(%q)\n got: %x\nwant: %x",
			strconv.FormatUint(uint64(crc), 10), block[192:], digest)
	}

	other, err := SignPayload([]byte(`[1,2,4]`))
	if err != nil {
		t.Fatalf("sign other payload: %v", err)
	}
	if other == sig {
		t.Fatal("different payloads produced the same signature")
	}
}

// writeKeyPair writes the two halves genCert produces, using the same encoding
// genCert uses, so the test exercises the real on-disk shapes.
func writeKeyPair(t *testing.T, dir, base string) (privatePath, publicPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privatePath = filepath.Join(dir, base+".pem")
	publicPath = filepath.Join(dir, base+".der")

	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write private: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public: %v", err)
	}
	if err := os.WriteFile(publicPath, pubDER, 0o644); err != nil {
		t.Fatalf("write public: %v", err)
	}
	return privatePath, publicPath
}

func TestLoadSigningKeyPair_AcceptsMatchingPair(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := writeKeyPair(t, dir, "sigkey")
	key, err := loadSigningKeyPair(privatePath, publicPath)
	if err != nil {
		t.Fatalf("matching pair should load, got %v", err)
	}
	if key.N.BitLen() != 2048 {
		t.Fatalf("key size = %d, want 2048", key.N.BitLen())
	}
}

func TestLoadSigningKeyPair_RejectsMissingPrivate(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := writeKeyPair(t, dir, "sigkey")
	if err := os.Remove(privatePath); err != nil {
		t.Fatal(err)
	}
	_, err := loadSigningKeyPair(privatePath, publicPath)
	if err == nil {
		t.Fatal("missing private key should be an error")
	}
	if !strings.Contains(err.Error(), "sigkey.pem") {
		t.Fatalf("error should name the private key, got %v", err)
	}
}

// The public half is what the client is made to trust. If it is absent the
// server would sign bodies nothing can validate, so it must be fatal.
func TestLoadSigningKeyPair_RejectsMissingPublic(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := writeKeyPair(t, dir, "sigkey")
	if err := os.Remove(publicPath); err != nil {
		t.Fatal(err)
	}
	_, err := loadSigningKeyPair(privatePath, publicPath)
	if err == nil {
		t.Fatal("missing public key should be an error")
	}
	if !strings.Contains(err.Error(), "sigkey.der") {
		t.Fatalf("error should name the public key, got %v", err)
	}
}

// Two halves from different generations produce signatures the client rejects,
// so a mismatch has to stop the server rather than be silently accepted.
func TestLoadSigningKeyPair_RejectsMismatchedPair(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := writeKeyPair(t, dir, "sigkey")
	otherDir := t.TempDir()
	_, otherPublic := writeKeyPair(t, otherDir, "other")
	other, err := os.ReadFile(otherPublic)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSigningKeyPair(privatePath, publicPath); err == nil {
		t.Fatal("a public key from a different pair should be an error")
	} else if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error should report a mismatch, got %v", err)
	}
}

func TestLoadSigningKeyPair_RejectsCorruptPrivate(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := writeKeyPair(t, dir, "sigkey")
	if err := os.WriteFile(privatePath, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSigningKeyPair(privatePath, publicPath); err == nil {
		t.Fatal("a corrupt private key should be an error")
	}
}
