package ops

import (
	"errors"
	"os"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

var errSigKeySetupFailed = errors.New("setup failed")

// fakeSigKeySetup runs in place of the config binary: it writes what it was told
// to write, or fails, and reports which options it was given.
func fakeSigKeySetup(t *testing.T, fail bool) (fn func(options *executor.ConfigSetupOptions) *exec.Result, seen *executor.ConfigSetupOptions) {
	t.Helper()
	seen = &executor.ConfigSetupOptions{}
	fn = func(options *executor.ConfigSetupOptions) *exec.Result {
		*seen = *options
		if fail {
			return &exec.Result{Err: errSigKeySetupFailed, ExitCode: 1}
		}
		if err := os.WriteFile(options.SigKeyFilePath, options.AddSigKeyData, 0600); err != nil {
			t.Fatalf("fake setup could not write: %v", err)
		}
		return &exec.Result{ExitCode: common.ErrSuccess}
	}
	return
}

// The key has to reach a file the client can open, with the exact bytes the
// server served: the client substitutes the key it trusts with this one, so a
// re-encoded or truncated copy would not match what the server signs with.
func TestAddSigKeyWritesTheKeyToAFileAndRemembersIt(t *testing.T) {
	origRead, origSetup := readSigningKeyFn, runSigKeySetupFn
	t.Cleanup(func() { readSigningKeyFn, runSigKeySetupFn = origRead, origSetup })

	key := []byte{0x30, 0x82, 0x01, 0x02, 0x03}
	readSigningKeyFn = func(string) []byte { return key }
	setupFn, seen := fakeSigKeySetup(t, false)
	runSigKeySetupFn = setupFn

	c := &Config{Report: launcher.Discard{}}
	c.gameId = "age2"
	defer func() {
		if c.sigKeyFilePath != "" {
			_ = os.Remove(c.sigKeyFilePath)
		}
	}()

	if exitCode := c.AddSigKey("127.0.0.1"); exitCode != common.ErrSuccess {
		t.Fatalf("AddSigKey = %d, want success", exitCode)
	}
	if c.sigKeyFilePath == "" {
		t.Fatal("the key path was not remembered, so the client would not be told where it is")
	}
	got, err := os.ReadFile(c.sigKeyFilePath)
	if err != nil {
		t.Fatalf("key file: %v", err)
	}
	if string(got) != string(key) {
		t.Fatalf("key file = %v, want %v", got, key)
	}
	if seen.SigKeyFilePath != c.sigKeyFilePath || string(seen.AddSigKeyData) != string(key) {
		t.Fatalf("setup got path=%q data=%v, want path=%q data=%v",
			seen.SigKeyFilePath, seen.AddSigKeyData, c.sigKeyFilePath, key)
	}
	if seen.GameId != "age2" {
		t.Errorf("setup got game %q, want age2", seen.GameId)
	}
}

// A setup that failed must not leave a key file behind: nothing points at it any
// more, and a later run would have no record to clean it with.
func TestAddSigKeyRemovesTheFileWhenSetupFails(t *testing.T) {
	origRead, origSetup := readSigningKeyFn, runSigKeySetupFn
	t.Cleanup(func() { readSigningKeyFn, runSigKeySetupFn = origRead, origSetup })

	readSigningKeyFn = func(string) []byte { return []byte{0x30, 0x01} }
	var writtenPath string
	runSigKeySetupFn = func(options *executor.ConfigSetupOptions) *exec.Result {
		writtenPath = options.SigKeyFilePath
		return &exec.Result{Err: errSigKeySetupFailed, ExitCode: 1}
	}

	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.AddSigKey("127.0.0.1"); exitCode != launcher.ErrConfigSigKeyAdd {
		t.Fatalf("AddSigKey = %d, want ErrConfigSigKeyAdd", exitCode)
	}
	if c.sigKeyFilePath != "" {
		t.Errorf("path %q survived a failed setup", c.sigKeyFilePath)
	}
	if writtenPath == "" {
		t.Fatal("setup was not asked to write anything")
	}
	if _, err := os.Stat(writtenPath); !os.IsNotExist(err) {
		t.Fatalf("temp key file %q survived: %v", writtenPath, err)
	}
}

// A server without a signing key is a normal state; the launcher must say so
// rather than pass an empty file to the client.
func TestAddSigKeyFailsWhenTheServerHasNoKey(t *testing.T) {
	origRead, origSetup := readSigningKeyFn, runSigKeySetupFn
	t.Cleanup(func() { readSigningKeyFn, runSigKeySetupFn = origRead, origSetup })

	readSigningKeyFn = func(string) []byte { return nil }
	runSigKeySetupFn = func(*executor.ConfigSetupOptions) *exec.Result {
		t.Error("setup ran without a key to write")
		return &exec.Result{ExitCode: common.ErrSuccess}
	}

	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.AddSigKey("127.0.0.1"); exitCode != launcher.ErrReadSigKey {
		t.Fatalf("AddSigKey = %d, want ErrReadSigKey", exitCode)
	}
	if c.sigKeyFilePath != "" {
		t.Errorf("a path was recorded without a key: %q", c.sigKeyFilePath)
	}
}
