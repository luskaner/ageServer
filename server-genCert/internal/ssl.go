package internal

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/luskaner/ageServer/common"
)

func generateSelfSignedCertificate(folder string) bool {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return false
	}
	template := getTemplate("selfsigned")
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return false
	}

	certFile, err := os.Create(filepath.Join(folder, common.SelfSignedCert))
	if err != nil {
		return false
	}
	var keyFile *os.File
	delCertFile := false
	delKeyFile := false
	defer func() {
		_ = certFile.Close()
		if delCertFile {
			_ = os.Remove(filepath.Join(folder, common.SelfSignedCert))
		}
		if keyFile != nil {
			_ = keyFile.Close()
			if delKeyFile {
				_ = os.Remove(filepath.Join(folder, common.SelfSignedKey))
			}
		}
	}()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	if err != nil {
		delCertFile = true
		return false
	}

	// Private key material must not be readable by group/other users.
	keyFile, err = os.OpenFile(filepath.Join(folder, common.SelfSignedKey), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)

	if err != nil {
		delCertFile = true
		return false
	}

	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		delCertFile = true
		delKeyFile = true
		return false
	}

	return true
}

func getTemplate(typ string) *x509.Certificate {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   common.Name,
			Organization: []string{common.CertSubjectOrganization},
		},
		// Backdate so clients with slight clock skew do not reject the
		// certificate as "not yet valid" right after generation.
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
	}
	if typ != "normal" {
		template.IsCA = true
	} else {
		template.DNSNames = common.CertDomains()
	}
	if typ == "selfsigned" {
		template.Subject.CommonName += " Self-signed"
		template.DNSNames = common.SelfSignedCertDomains
		template.MaxPathLenZero = true
	} else if typ == "ca" {
		template.Subject.CommonName += " CA"
	}
	if typ != "normal" {
		template.KeyUsage = x509.KeyUsageCertSign
	}
	if typ != "ca" {
		template.KeyUsage |= x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	if typ == "selfsigned" {
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	}
	return template
}

func generateCertificatePairs(folder string, certName string, keyName string, parent *x509.Certificate, parentKey *rsa.PrivateKey) (ok bool, caKey *bytes.Buffer) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	var typ string
	if parent == nil && parentKey == nil {
		typ = "ca"
	} else {
		typ = "normal"
	}
	template := getTemplate(typ)
	if typ == "ca" {
		parent = template
		parentKey = key
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		return
	}
	certFile, err := os.Create(filepath.Join(folder, certName))
	if err != nil {
		return
	}
	var keyFile *os.File
	delCertFile := false
	delKeyFile := false
	defer func() {
		_ = certFile.Close()
		if delCertFile {
			_ = os.Remove(filepath.Join(folder, certName))
		}
		if keyFile != nil {
			_ = keyFile.Close()
			if delKeyFile {
				_ = os.Remove(filepath.Join(folder, keyName))
			}
		}
	}()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	if err != nil {
		delCertFile = true
		return
	}

	var outputKey io.Writer
	if keyName != "" {
		// Private key material must not be readable by group/other users.
		keyFile, err = os.OpenFile(filepath.Join(folder, keyName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			delCertFile = true
			return
		}
		outputKey = keyFile
	} else {
		caKey = &bytes.Buffer{}
		outputKey = caKey
	}

	err = pem.Encode(outputKey, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		delCertFile = true
		delKeyFile = true
		return
	}

	ok = true
	return
}

// generateSigningKeyPair creates the RSA-2048 keypair RelicLink's dataSignature
// is verified against.
//
// The private half stays here and signs the catalogue bodies the server serves.
// The public half is written as SPKI DER next to it, and is what the launcher
// companion hands to the client so it trusts our signatures instead of
// Microsoft's.
//
// Two properties are load-bearing and are asserted before writing anything:
//
//   - 2048 bits, because the client passes a fixed length of 294 bytes to its
//     key parser and anything else is rejected;
//   - an SPKI of exactly 294 bytes, so the replacement is size-preserving and
//     does not disturb the image layout.
//
// Returns false after removing any partial output.
func generateSigningKeyPair(folder string) (ok bool) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return false
	}
	if privateKey.N.BitLen() != 2048 || privateKey.E != 65537 {
		return false
	}

	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return false
	}
	// 30 82 01 22 = SEQUENCE, 0x122 = 290 plus a 4-byte header.
	if len(der) != 294 || der[0] != 0x30 || der[1] != 0x82 || der[2] != 0x01 || der[3] != 0x22 {
		return false
	}

	privPath := filepath.Join(folder, common.SigningKey)
	pubPath := filepath.Join(folder, common.SigningPublicKey)
	ok = false
	defer func() {
		if !ok {
			_ = os.Remove(privPath)
			_ = os.Remove(pubPath)
		}
	}()

	// Private key material must not be readable by group/other users.
	keyFile, err := os.OpenFile(privPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return false
	}
	defer func() { _ = keyFile.Close() }()

	if err = pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		return false
	}

	// The public half is not secret; it is meant to be shipped with the client.
	pubFile, err := os.OpenFile(pubPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return false
	}
	defer func() { _ = pubFile.Close() }()

	if _, err = pubFile.Write(der); err != nil {
		return false
	}

	ok = true
	return
}

func GenerateCertificatePairs(folder string) (ok bool) {
	var caKey *bytes.Buffer
	if ok, caKey = generateCertificatePairs(folder, common.CACert, "", nil, nil); !ok {
		return
	}
	ok = false
	defer func() {
		if !ok {
			_ = os.Remove(filepath.Join(folder, common.Key))
			_ = os.Remove(filepath.Join(folder, common.Cert))
		}
	}()

	certFile, err := os.ReadFile(filepath.Join(folder, common.CACert))
	if err != nil {
		return
	}

	fullCert, err := tls.X509KeyPair(certFile, caKey.Bytes())
	if err != nil {
		return
	}

	fullCert.Leaf, err = x509.ParseCertificate(fullCert.Certificate[0])
	if err != nil || fullCert.Leaf == nil {
		return
	}

	if ok, _ = generateCertificatePairs(
		folder, common.Cert, common.Key, fullCert.Leaf, fullCert.PrivateKey.(*rsa.PrivateKey)); !ok {
		_ = os.Remove(filepath.Join(folder, common.CACert))
		return
	}

	if ok = generateSelfSignedCertificate(folder); !ok {
		_ = os.Remove(filepath.Join(folder, common.CACert))
		return
	}

	// The dataSignature keypair is separate from the TLS material above and is
	// not part of CertificatePairs: without it the server still runs, it just
	// falls back to replaying captured bodies instead of signing its own.
	if ok = generateSigningKeyPair(folder); !ok {
		_ = os.Remove(filepath.Join(folder, common.CACert))
		return
	}

	ok = true
	return
}
