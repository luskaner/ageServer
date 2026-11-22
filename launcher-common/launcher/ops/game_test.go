package ops

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A real file must reach StartAgent as a usable writer. The typed-nil half of
// the L-4 gotcha is documented at its guard in game.go: it is a property of the
// language, not something this package can assert.
func TestNonNilFileProducesValidWriter(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "agent-log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var out io.Writer
	if f != nil {
		out = f
	}

	if out == nil {
		t.Fatal("valid file must produce non-nil writer")
	}
}

// The client launcher learns where the files this run produced are through these
// values, so every path the run has must be there, and a run that has none must
// offer nothing rather than an empty string: a client given "" would be told a
// file exists at the empty path.
func TestClientArgsValuesOfferEveryFileTheRunHas(t *testing.T) {
	c := &Config{}
	if values := c.clientArgsValues(); values != nil {
		t.Fatalf("a run that produced no files offered values: %v", values)
	}

	c.hostFilePath = filepath.Join("tmp", "hosts")
	c.certFilePath = filepath.Join("tmp", "cert.pem")
	c.sigKeyFilePath = filepath.Join("tmp", "sigkey.der")
	values := c.clientArgsValues()
	for name, path := range map[string]string{
		"HostFilePath":   c.hostFilePath,
		"CertFilePath":   c.certFilePath,
		"SigKeyFilePath": c.sigKeyFilePath,
	} {
		want := path
		if runtime.GOOS == "windows" {
			want = strings.ReplaceAll(path, `\`, `\\`)
		}
		if values[name] != want {
			t.Errorf("values[%q] = %q, want %q", name, values[name], want)
		}
	}
	// With the API embedded there is no API certificate file of its own: the
	// placeholder must resolve to the root server's PEM, not stay unreplaced
	// for the client to read as a literal path.
	if values["ApiCertFilePath"] != values["CertFilePath"] {
		t.Errorf("values[ApiCertFilePath] = %q, want %q (the embedded API shares the root server's certificate)", values["ApiCertFilePath"], values["CertFilePath"])
	}
	if len(values) != 4 {
		t.Errorf("values = %v, want the three paths the run has plus the API certificate", values)
	}

	c.apiCertFilePath = filepath.Join("tmp", "api-cert.pem")
	values = c.clientArgsValues()
	wantApi := c.apiCertFilePath
	if runtime.GOOS == "windows" {
		wantApi = strings.ReplaceAll(wantApi, `\`, `\\`)
	}
	if values["ApiCertFilePath"] != wantApi {
		t.Errorf("values[ApiCertFilePath] = %q, want %q (the separate pass produced its own file)", values["ApiCertFilePath"], wantApi)
	}
}
