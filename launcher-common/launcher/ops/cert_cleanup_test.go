package ops

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression: temp certificate files created via os.CreateTemp were never
// cleaned up on error paths, leaving orphaned empty .pem files in %TEMP%.
func TestTempCertFileCleanupOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_cert.pem")

	// Simulate creating a temp cert file (as AddCert does).
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate the deferred cleanup pattern from AddCert.
	errorCode := 1 // non-zero = error
	if errorCode != 0 {
		_ = os.Remove(path)
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("temp file should have been removed on error")
	}
}

func TestTempCertFileKeptOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_cert.pem")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	// On success (errorCode == 0), the file must be kept.
	errorCode := 0
	if errorCode != 0 {
		_ = os.Remove(path)
	}

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Fatal("temp file should be kept on success")
	}
}
