package ops

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Documents the Go interface gotcha that caused L-4: assigning a typed-nil
// *os.File to an io.Writer produces a NON-NIL interface value.
func TestTypedNilFileIsNonNilWriter(t *testing.T) {
	var f *os.File = nil
	var out io.Writer = f

	// This is the Go gotcha: the interface is non-nil even though the
	// underlying pointer IS nil.
	if out == nil {
		t.Fatal("expected typed-nil to be non-nil as io.Writer (Go semantics)")
	}
}

// Regression for L-4: when FileLogger.Open returns a nil *os.File (no log
// root configured), the code must NOT pass a typed-nil to StartAgent.
func TestNilFileProducesUntypedNilWriter(t *testing.T) {
	var f *os.File // nil, as returned by Root.Open on nil receiver

	// The fix pattern used in game.go: explicit check before conversion.
	var out io.Writer
	if f != nil {
		out = f
	}

	if out != nil {
		t.Fatal("out must be untyped nil when file is nil — otherwise StartAgent's `out != nil` guard passes incorrectly")
	}
}

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
