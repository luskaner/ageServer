package models

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/luskaner/ageServer/common"
)

// RelicLink verifies catalogue bodies with an RSA-2048 signature carried in
// "dataSignature". The scheme was recovered from the client and is NOT standard
// PKCS#1 v1.5 in one respect: the block that gets signed contains the bare
// SHA-512 digest, with no ASN.1 DigestInfo prefix, and the FF padding is
// maximum length.
//
//	signature = base64( ( 0x00 || 0x01 || FF*189 || 0x00 || SHA-512(tag) ) ^ d mod n )
//	tag       = decimal ASCII of crc32(body with the dataSignature value emptied)
//
// The body is used verbatim apart from emptying that one value - no
// re-serialisation, no field reordering, no whitespace normalisation. The
// client recovers the same bytes by locating the signature inside the body with
// strstr and splicing it out, so the two sides agree by construction.
//
// This only works when the client trusts our key. The key that does the trusting
// lives in the game process, so FakeSignature.dll substitutes it at run time;
// without that DLL the client still verifies against Microsoft's key and will
// reject anything signed here.
const (
	// sigkey.pem is written by genCert into the same resources/certificates
	// folder as the TLS material, and is looked up there. Its public half,
	// sigkey.der, is what the launcher companion hands to the client.
	signatureLength = 344
)

// Matches the dataSignature field, capturing the value so it can be read and
// replaced independently. Group 1 is the opening quote and key, group 2 the
// value, group 3 the closing quote.
var dataSignatureField = regexp.MustCompile(`("dataSignature"\s*:\s*")([^"]*)(")`)

var (
	signingKeyOnce sync.Once
	signingKey     *rsa.PrivateKey
	signingKeyOK   bool
	signingKeyErr  error
)

// SigningKeyConfigured reports whether the signing key pair genCert writes is
// present, loadable and self-consistent. Both halves are required: the private
// key is what signs here, and sigkey.der is what the client is made to trust, so
// an install missing either one cannot work.
func SigningKeyConfigured() bool {
	signingKeyOnce.Do(loadSigningKey)
	return signingKeyOK
}

// SigningKeyError returns the reason the key pair could not be loaded, naming
// the offending file so the failure is actionable from the log alone.
func SigningKeyError() error {
	signingKeyOnce.Do(loadSigningKey)
	return signingKeyErr
}

// signingKeyPaths returns the two halves genCert writes into the same
// resources/certificates folder as the TLS material, resolved through the shared
// helper so the two can never drift apart. No flag and no environment variable:
// the key sits next to the certificates.
func signingKeyPaths() (privatePath string, publicPath string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	folder := common.CertificatePairFolder(exe)
	if folder == "" {
		return "", "", fmt.Errorf("no certificate folder next to %s", exe)
	}
	return filepath.Join(folder, common.SigningKey),
		filepath.Join(folder, common.SigningPublicKey), nil
}

func loadSigningKey() {
	privatePath, publicPath, err := signingKeyPaths()
	if err != nil {
		signingKeyErr = err
		return
	}
	key, err := loadSigningKeyPair(privatePath, publicPath)
	if err != nil {
		signingKeyErr = err
		return
	}
	signingKey = key
	signingKeyOK = true
}

// CheckSigningKeyPair verifies that folder holds a usable signing key pair.
// Exported so the entry point can check the same folder it already resolved for
// the TLS material and report a clean failure, rather than letting the panic in
// initializeResponses surface later.
func CheckSigningKeyPair(folder string) error {
	if folder == "" {
		return fmt.Errorf("no certificate folder to look for %s in", common.SigningKey)
	}
	_, err := loadSigningKeyPair(
		filepath.Join(folder, common.SigningKey),
		filepath.Join(folder, common.SigningPublicKey))
	return err
}

// loadSigningKeyPair loads both halves and checks they belong together. Split out
// from loadSigningKey so it can be tested against real files: the paths the
// server itself uses come from os.Executable and cannot be redirected.
func loadSigningKeyPair(privatePath, publicPath string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, fmt.Errorf("cannot sign without %s: %w", privatePath, err)
	}
	key, err := parseRSAPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", privatePath, err)
	}
	// The client only ever sees sigkey.der, via the launcher companion. If the
	// halves disagree the server would happily sign bodies the client rejects,
	// which is far worse than refusing to start, so cross-check them.
	pubDER, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, fmt.Errorf("cannot sign without %s: %w", publicPath, err)
	}
	parsed, err := x509.ParsePKIXPublicKey(pubDER)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", publicPath, err)
	}
	pub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an RSA public key", publicPath)
	}
	if pub.E != key.PublicKey.E || pub.N.Cmp(key.PublicKey.N) != 0 {
		return nil, fmt.Errorf("%s does not match the private key in %s",
			publicPath, privatePath)
	}
	return key, nil
}

func parseRSAPrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("not PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return validateRSA(key)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA key")
	}
	return validateRSA(key)
}

// The SPKI the client embeds is RSA-2048, and it passes 294 to the parser as a
// fixed size, so anything else would never be parsed in place of it.
func validateRSA(key *rsa.PrivateKey) (*rsa.PrivateKey, error) {
	if key.N.BitLen() != 2048 {
		return nil, fmt.Errorf("expected 2048 bits, got %d", key.N.BitLen())
	}
	if key.E != 65537 {
		return nil, fmt.Errorf("expected exponent 65537, got %d", key.E)
	}
	return key, nil
}

// SignTag signs the decimal crc32 the client will derive from body.
//
// crypto.Hash(0) is what makes this correct: it tells the rsa package to sign
// the 64 bytes as given instead of prepending an ASN.1 DigestInfo, which is
// exactly the variant RelicLink uses. The resulting block is
// 0x00 0x01 FF...FF 0x00 digest with maximum-length padding.
func signTag(crc uint32) (string, error) {
	signingKeyOnce.Do(loadSigningKey)
	if !signingKeyOK {
		// Always a non-nil error: an empty signature with a nil error would look
		// like a successful signing and be served to the client.
		if signingKeyErr != nil {
			return "", signingKeyErr
		}
		return "", fmt.Errorf("no signing key configured")
	}
	tag := strconv.FormatUint(uint64(crc), 10)
	digest := sha512.Sum512([]byte(tag))
	raw, err := signingKey.Sign(rand.Reader, digest[:], crypto.Hash(0))
	if err != nil {
		return "", err
	}
	wire := base64.StdEncoding.EncodeToString(raw)
	if len(wire) != signatureLength {
		return "", fmt.Errorf("unexpected signature length %d", len(wire))
	}
	return wire, nil
}

// StripSignature empties the dataSignature value, leaving every other byte
// untouched. This is the exact byte sequence the client hashes.
func StripSignature(body []byte) []byte {
	return dataSignatureField.ReplaceAll(body, []byte(`${1}${3}`))
}

// StampSignature returns body with a freshly signed dataSignature, plus the
// signature it wrote. Idempotent: re-stamping a stamped body yields the same
// result, because the CRC is derived from the stripped form.
//
// A body that cannot be signed is refused rather than substituted with
// something else: the client only accepts a signature it can verify, so a
// placeholder would turn a broken install into a failure that only shows up at
// verification time, on the client, with nothing in the server log.
func StampSignature(body []byte) ([]byte, string, error) {
	stripped := StripSignature(body)
	crc := crc32.ChecksumIEEE(stripped)

	sig, err := signTag(crc)
	if err != nil {
		return nil, "", fmt.Errorf("cannot sign body: %w", err)
	}
	stamped := dataSignatureField.ReplaceAll(stripped, []byte(`${1}`+sig+`${3}`))
	return stamped, sig, nil
}

// SignatureOf extracts the dataSignature from a body.
func SignatureOf(body []byte) string {
	probe := dataSignatureField.FindSubmatch(body)
	if probe == nil {
		return ""
	}
	return string(probe[2])
}

// SignPayload signs a payload the caller already holds verbatim, using the same
// scheme as a catalogue body: crc32 of the payload, decimal ASCII, SHA-512, and
// the same signature block.
//
// Taking the caller's bytes rather than a struct keeps the contract honest. A
// catalogue body is signed over itself, so the client can recover the exact
// bytes by splicing the signature out of what it already received; anything
// signed from a Go value would need the client to re-serialise identically,
// which is a coupling this scheme does not otherwise have.
func SignPayload(payload []byte) (string, error) {
	return signTag(crc32.ChecksumIEEE(payload))
}
