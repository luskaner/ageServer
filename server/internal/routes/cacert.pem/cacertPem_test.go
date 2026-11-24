package cacert_pem

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
)

func TestCacertPemNotFoundWhenNoFile(t *testing.T) {
	req := httptest.NewRequest("GET", "/cacert.pem/root", nil)
	rr := httptest.NewRecorder()
	CacertPem(rr, req, common.CertModeModern)
	// The test binary's certificate folder normally has no cert.pem.
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCacertPemServesTheFileOfTheMode(t *testing.T) {
	exe, _ := os.Executable()
	folder := common.CertificatePairFolder(exe)
	if folder == "" {
		t.Skip("could not determine folder")
	}
	for file, contents := range map[string]string{
		common.CACert:         "modern-ca",
		common.SelfSignedCert: "legacy-ca",
	} {
		path := filepath.Join(folder, file)
		if _, err := os.Stat(path); err == nil {
			// Do not overwrite a real certificate pair of the machine.
			t.Skipf("%s already exists", path)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
	}
	for mode, file := range map[common.CertMode]string{
		common.CertModeModern: common.CACert,
		common.CertModeLegacy: common.SelfSignedCert,
	} {
		req := httptest.NewRequest("GET", "/cacert.pem/root", nil)
		rr := httptest.NewRecorder()
		CacertPem(rr, req, mode)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", mode, rr.Code)
		}
		want, _ := os.ReadFile(filepath.Join(folder, file))
		if rr.Body.String() != string(want) {
			t.Fatalf("%s served %q, want the contents of %s", mode, rr.Body.String(), file)
		}
	}
}
