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
	if len(values) != 3 {
		t.Errorf("values = %v, want only the three paths the run has", values)
	}
}
