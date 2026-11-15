package ops

import (
	"crypto/x509"
	"errors"
	"os"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

var errCertSetupFailed = errors.New("cert setup failed")

// Regression: temp certificate files created via os.CreateTemp were never
// cleaned up on error paths, leaving orphaned empty .pem files in %TEMP%.
func TestAddCertRemovesTempFileWhenSetupFails(t *testing.T) {
	origSetup := runCertSetupFn
	t.Cleanup(func() { runCertSetupFn = origSetup })
	var writtenPath string
	runCertSetupFn = func(options *executor.ConfigSetupOptions) *exec.Result {
		writtenPath = options.CertFilePath
		return &exec.Result{Err: errCertSetupFailed, ExitCode: 1}
	}

	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.AddCert("age2", uuid.Nil(), &x509.Certificate{Raw: []byte{0x30, 0x01}}, "local", true, false); exitCode != launcher.ErrConfigCertAdd {
		t.Fatalf("AddCert = %d, want ErrConfigCertAdd", exitCode)
	}
	if writtenPath == "" {
		t.Fatal("setup was not asked to write a certificate file")
	}
	if _, err := os.Stat(writtenPath); !os.IsNotExist(err) {
		t.Fatalf("temp certificate file %q survived: %v", writtenPath, err)
	}
}

// A setup that succeeded produced the file the client is told to read, so it
// must stay where the launcher pointed it.
func TestAddCertKeepsTempFileOnSuccess(t *testing.T) {
	origSetup := runCertSetupFn
	t.Cleanup(func() { runCertSetupFn = origSetup })
	runCertSetupFn = func(*executor.ConfigSetupOptions) *exec.Result { return &exec.Result{} }

	c := &Config{Report: launcher.Discard{}}
	exitCode := c.AddCert("age2", uuid.Nil(), &x509.Certificate{Raw: []byte{0x30, 0x01}}, "local", true, false)
	if exitCode != common.ErrSuccess {
		t.Fatalf("AddCert = %d, want success", exitCode)
	}
	if c.certFilePath == "" {
		t.Fatal("the certificate path was not remembered, so the client would not be told where it is")
	}
	t.Cleanup(func() { _ = os.Remove(c.certFilePath) })
	if _, err := os.Stat(c.certFilePath); err != nil {
		t.Fatalf("certificate file %q missing after a successful setup: %v", c.certFilePath, err)
	}
}
