package server

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmd/genCert"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/common/server"
)

// signingKeySize is the length of an RSA-2048 SubjectPublicKeyInfo, and the size
// the client's key substitution is hardcoded to: the signing key DLL overwrites
// the 294-byte key already embedded in the game and refuses a file of any other
// length. A key that does not match this is not one the client can use, so it is
// rejected here rather than passed on to fail silently in the game.
const signingKeySize = 294

// fetchFromServer downloads path from the server at host over HTTPS, without
// verifying its certificate.
//
// Verification is deliberately skipped: the only thing worth verifying against is
// the server's own certificate, which is one of the files fetched here. What the
// downloads are used for is then checked in its own terms -a certificate has to
// parse as one, a signing key as a public key- so a wrong server answering still
// cannot pass a certificate or key off as valid.
func fetchFromServer(host string, path string) (body []byte, err error) {
	tr := &http.Transport{
		TLSClientConfig: server.TlsConfig(host, true, nil),
	}
	ips := common.HostOrIpToIps(host)
	var ip string
	if len(ips) == 0 {
		ip = host
	} else {
		ip = ips[0]
	}
	client := &http.Client{Transport: tr, Timeout: 1 * time.Second}
	var req *http.Request
	if req, err = http.NewRequest("GET", fmt.Sprintf("https://%s/%s", ip, path), nil); err != nil {
		return
	}
	req.Header.Set("User-Agent", common.UserAgent())
	req.Host = host
	//goland:noinspection ALL
	var resp *http.Response
	if resp, err = client.Do(req); err != nil {
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func ReadCACertificateFromServer(host string) *x509.Certificate {
	bodyBytes, err := fetchFromServer(host, common.CACert)
	if err != nil {
		commonLogger.Println("ReadCACertificateFromServer error:", err)
		return nil
	}
	return parseCertFromPEM(bodyBytes)
}

// ReadSigningKeyFromServer returns the public signing key the server serves at
// sigkey.der, verbatim, or nil if it could not be fetched or is not a key the
// client can use.
//
// The bytes are returned as served rather than re-encoded: the client is pointed
// at this file so it can substitute the key it trusts, so it has to be the same
// DER the server signs with. Re-encoding would produce an equivalent key but an
// unnecessary chance to disagree with the server.
func ReadSigningKeyFromServer(host string) []byte {
	bodyBytes, err := fetchFromServer(host, common.SigningPublicKey)
	if err != nil {
		commonLogger.Println("ReadSigningKeyFromServer error:", err)
		return nil
	}
	pub, err := x509.ParsePKIXPublicKey(bodyBytes)
	if err != nil {
		commonLogger.Println("ReadSigningKeyFromServer parse error:", err)
		return nil
	}
	if _, ok := pub.(*rsa.PublicKey); !ok || len(bodyBytes) != signingKeySize {
		commonLogger.Println("ReadSigningKeyFromServer error:",
			fmt.Errorf("expected an RSA key of %d bytes, got %d", signingKeySize, len(bodyBytes)))
		return nil
	}
	return bodyBytes
}

func GenerateCertificatePair(certificateFolder string, optionsFn func(options *exec.Options)) (result *exec.Result) {
	baseFolder := filepath.Join(certificateFolder, "..", "..")
	exePath := filepath.Join(baseFolder, executables.NativeFileName(false, executables.ServerGenCert))
	if _, err := os.Stat(exePath); err != nil {
		return nil
	}
	values, singleFs := genCert.SingleFlagSet("", nil)
	values.Replace = true
	options := exec.Options{File: exePath, Wait: true, Args: cmd.FlagSetToArgs(singleFs.Fs(), false), ExitCode: true}
	if optionsFn != nil {
		optionsFn(&options)
	}
	result = options.Exec()
	return
}

func CertificateSoonExpired(cert string) bool {
	if cert == "" {
		return true
	}

	certPEM, err := os.ReadFile(cert)
	if err != nil {
		return true
	}

	return certSoonExpiredFromBytes(certPEM, time.Now())
}

func parseCertFromPEM(bodyBytes []byte) *x509.Certificate {
	block, _ := pem.Decode(bodyBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		commonLogger.Println("ReadCACertificateFromServer: no certificate found")
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		commonLogger.Println("ReadCACertificateFromServer parse error:", err)
		return nil
	}
	return cert
}

func certSoonExpiredFromBytes(certPEM []byte, now time.Time) bool {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return true
	}

	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true
	}

	return now.Add(24 * time.Hour).After(crt.NotAfter)
}
