package ops

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

// The API certificate is a second certificate file a custom client can read,
// so the pass has to keep its path and hand it to the client arguments.
func TestAddApiCertKeepsItsFileAndExposesThePlaceholder(t *testing.T) {
	origSetup := runCertSetupFn
	t.Cleanup(func() { runCertSetupFn = origSetup })
	var writtenPath string
	runCertSetupFn = func(options *executor.ConfigSetupOptions) *exec.Result {
		writtenPath = options.ApiCertFilePath
		if options.CertFilePath != "" {
			t.Errorf("the API pass wrote the root certificate path %q", options.CertFilePath)
		}
		return &exec.Result{}
	}

	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.AddApiCert("age2", &x509.Certificate{Raw: []byte{0x30, 0x01}}, "local", true, false); exitCode != common.ErrSuccess {
		t.Fatalf("AddApiCert = %d, want success", exitCode)
	}
	if c.apiCertFilePath == "" {
		t.Fatal("the API certificate path was not remembered")
	}
	if writtenPath != c.apiCertFilePath {
		t.Fatalf("setup wrote %q, but the client would be told %q", writtenPath, c.apiCertFilePath)
	}
	t.Cleanup(func() { _ = os.Remove(c.apiCertFilePath) })
	if _, err := os.Stat(c.apiCertFilePath); err != nil {
		t.Fatalf("certificate file %q missing after a successful setup: %v", c.apiCertFilePath, err)
	}
	if got := c.clientArgsValues()["ApiCertFilePath"]; got == "" {
		t.Fatal("the client arguments do not expose {ApiCertFilePath}")
	}
}

// A failed setup must not leave the empty temporary file behind, the same way
// the root certificate pass does not.
func TestAddApiCertRemovesItsTempFileWhenSetupFails(t *testing.T) {
	origSetup := runCertSetupFn
	t.Cleanup(func() { runCertSetupFn = origSetup })
	var writtenPath string
	runCertSetupFn = func(options *executor.ConfigSetupOptions) *exec.Result {
		writtenPath = options.ApiCertFilePath
		return &exec.Result{Err: errCertSetupFailed, ExitCode: 1}
	}

	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.AddApiCert("age2", &x509.Certificate{Raw: []byte{0x30, 0x01}}, "local", true, false); exitCode != launcher.ErrConfigCertAdd {
		t.Fatalf("AddApiCert = %d, want ErrConfigCertAdd", exitCode)
	}
	if writtenPath == "" {
		t.Fatal("setup was not asked to write a certificate file")
	}
	if _, err := os.Stat(writtenPath); !os.IsNotExist(err) {
		t.Fatalf("temp certificate file %q survived: %v", writtenPath, err)
	}
}

// With a separate API server the root certificate pass must not look at the
// API domains: they resolve to the standalone API server, which serves
// neither the announcement nor /cacert.pem/root. The two halves also have to
// stay disjoint and, together, the whole set.
func TestRootCertDomainsLeaveTheApiDomainsToTheirPass(t *testing.T) {
	whole := rootCertDomains(game.AoE2, false, false)
	api := common.ApiHosts(game.AoE2, false)
	if len(api) == 0 {
		t.Fatal("the game does not publish any API domain, so the split cannot be exercised")
	}
	for _, domain := range rootCertDomains(game.AoE2, false, true) {
		for _, apiDomain := range api {
			if domain == apiDomain {
				t.Fatalf("the root pass looks at the API domain %s although the API is separate", domain)
			}
		}
	}
	counted := make(map[string]int)
	for _, domain := range append(append([]string{}, rootCertDomains(game.AoE2, false, true)...), api...) {
		counted[domain]++
	}
	if len(counted) != len(whole) {
		t.Fatalf("root + API domains (%d distinct) do not cover the whole set (%d)", len(counted), len(whole))
	}
}

// The game's own CA bundle is what decides, by content and not by resolving
// the API domains: with a {HostFilePath} client the system resolver knows
// nothing about the session's mapping at this point, so a connection check
// would re-append the certificate on every run.
func TestAddApiCACertToGameAddsOnceAndSkipsWhatIsAlreadyThere(t *testing.T) {
	origSetup := runCertSetupFn
	t.Cleanup(func() { runCertSetupFn = origSetup })
	gamePath := t.TempDir()
	bundleCert, apiCert := generatedCACert(t, "game bundle root"), generatedCACert(t, "api root")

	t.Run("adds when the bundle does not hold the API certificate", func(t *testing.T) {
		bundlePath := writeBundleFile(t, bundleCert)
		var addedCert *x509.Certificate
		var usedGamePath string
		runCertSetupFn = func(options *executor.ConfigSetupOptions) *exec.Result {
			var parseErr error
			addedCert, parseErr = x509.ParseCertificate(options.AddCACertData)
			if parseErr != nil {
				t.Fatalf("setup was not given a parseable certificate: %v", parseErr)
			}
			if options.CertFilePath != "" {
				t.Errorf("an API CA pass with a game bundle wrote %q", options.CertFilePath)
			}
			usedGamePath = options.GamePath
			return &exec.Result{}
		}
		c := &Config{Report: launcher.Discard{}}
		if exitCode := c.AddApiCACertToGame(game.AoE2, apiCert, gamePath, bundlePath, true, false); exitCode != common.ErrSuccess {
			t.Fatalf("AddApiCACertToGame = %d, want success", exitCode)
		}
		if addedCert == nil {
			t.Fatal("no certificate reached the game bundle")
		}
		if usedGamePath != gamePath {
			t.Fatalf("setup used game path %q, want %q", usedGamePath, gamePath)
		}
	})

	t.Run("skips when the bundle already holds it", func(t *testing.T) {
		bundlePath := writeBundleFile(t, bundleCert, apiCert)
		runCertSetupFn = func(*executor.ConfigSetupOptions) *exec.Result {
			t.Error("the certificate was written although the bundle already holds it")
			return &exec.Result{}
		}
		c := &Config{Report: launcher.Discard{}}
		if exitCode := c.AddApiCACertToGame(game.AoE2, apiCert, gamePath, bundlePath, true, false); exitCode != common.ErrSuccess {
			t.Fatalf("AddApiCACertToGame = %d, want success", exitCode)
		}
	})

	t.Run("refuses without permission", func(t *testing.T) {
		bundlePath := writeBundleFile(t, bundleCert)
		c := &Config{Report: launcher.Discard{}}
		if exitCode := c.AddApiCACertToGame(game.AoE2, apiCert, gamePath, bundlePath, false, false); exitCode != launcher.ErrConfigCert {
			t.Fatalf("AddApiCACertToGame = %d, want ErrConfigCert", exitCode)
		}
	})
}

// writeBundleFile writes a readable PEM file with the given certificates, for
// the bundle-reading part of the game CA checks.
func writeBundleFile(t *testing.T, certs ...*x509.Certificate) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "bundle-*.pem")
	if err != nil {
		t.Fatalf("failed to create the bundle file: %v", err)
	}
	defer func() { _ = file.Close() }()
	for _, cert := range certs {
		if err = common.WriteAsPem(cert.Raw, file); err != nil {
			t.Fatalf("failed to write the bundle file: %v", err)
		}
	}
	return file.Name()
}

func generatedCACert(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()&0x7fffffff + 1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate the CA key: %v", err)
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create the CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse the CA certificate: %v", err)
	}
	return cert
}
