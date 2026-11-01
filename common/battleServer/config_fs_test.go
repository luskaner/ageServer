package battleServer

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: a single unparseable file (e.g. notes.txt) in the folder used to
// poison the named-return err of Configs, making valid configs unusable.
func TestConfigsIgnoresForeignFiles(t *testing.T) {
	gameId := "unit-" + strings.TrimPrefix(filepath.Base(t.TempDir()), "Test")
	dir := Folder(gameId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(dir)) })

	validToml := "Region = \"region-1\"\nIPv4 = \"127.0.0.1\"\nBsPort = 1\nWebSocketPort = 2\n"
	if err := os.WriteFile(filepath.Join(dir, Name(0)), []byte(validToml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("unrelated junk"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	configs, err := Configs(gameId, false, true)
	if err != nil {
		t.Fatalf("foreign file must not fail the listing: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("configs = %d, want 1", len(configs))
	}
	if configs[0].index != 0 {
		t.Fatalf("index = %d, want 0", configs[0].index)
	}
	if configs[0].Region != "region-1" || configs[0].IPv4 != "127.0.0.1" || configs[0].BsPort != 1 || configs[0].WebSocketPort != 2 {
		t.Fatalf("parsed config mismatch: %+v", configs[0].Base)
	}
}

func TestConfigsMissingFolderIsNotAnError(t *testing.T) {
	configs, err := Configs("unit-never-created-xyz", false, true)
	if err != nil {
		t.Fatalf("missing folder must return nil error, got %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("configs = %d, want 0", len(configs))
	}
}

func TestConfigsBrokenTomlFailsWithFileName(t *testing.T) {
	gameId := "unit-broken"
	dir := Folder(gameId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, Name(7)), []byte("%%%% not toml %%"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Configs(gameId, false, true)
	if err == nil {
		t.Fatal("expected parse error for broken toml")
	}
	if !strings.Contains(err.Error(), Name(7)) {
		t.Fatalf("error must mention the offending file: %v", err)
	}
}

func TestConfigsMissingFolderErrorIsNilEvenWithOnlyValid(t *testing.T) {
	// onlyValid=true must not change the missing-folder behavior.
	configs, err := Configs("unit-never-created-abc", true, false)
	if err != nil || len(configs) != 0 {
		t.Fatalf("got %v, %d", err, len(configs))
	}
}

func TestParseFileNameRejectsNonNumeric(t *testing.T) {
	if _, err := ParseFileName("config.toml"); err == nil {
		t.Fatal("expected error for alphabetic name")
	}
	if _, err := ParseFileName(""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestConfigsReadDirPermissionError(t *testing.T) {
	// On Windows as admin, many permission tricks don't work.
	// Use an extremely long path to force os.ReadDir to fail.
	gameId := strings.Repeat("x", 300)
	_, err := Configs(gameId, false, true)
	if err == nil {
		t.Skip("ReadDir did not error on long path (platform-specific)")
	}
	if !strings.Contains(err.Error(), "error while reading battle servers config directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigsReadFilePermissionError(t *testing.T) {
	// Create a valid TOML file, then truncate it to empty so toml.Unmarshal
	// will fail with a parse error (covers the ReadFile success → unmarshal error path).
	// Also test with a symlink pointing to a deleted file for actual ReadFile error.
	gameId := "unit-file-err"
	dir := Folder(gameId)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// Test symlink to nonexistent target → ReadFile error
	badLink := filepath.Join(dir, Name(99))
	if err := os.Symlink(filepath.Join(t.TempDir(), "nonexistent"), badLink); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	_, err := Configs(gameId, false, true)
	if err == nil {
		t.Fatal("expected error for broken symlink")
	}
	if !strings.Contains(err.Error(), "error while reading battle server config file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigPath(t *testing.T) {
	c := Config{index: 5}
	if got := c.Path(); got != "5.toml" {
		t.Errorf("Path() = %q, want %q", got, "5.toml")
	}
}

func TestConfigsValidOnlyWithDialSuccess(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	gameId := "unit-valid-only"
	dir := Folder(gameId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	toml := fmt.Sprintf(`Region = "eu"
Name = "test"
IPv4 = "%s"
BsPort = %d
WebSocketPort = %d
PID = 1
`, addr.IP, addr.Port, addr.Port)
	if err := os.WriteFile(filepath.Join(dir, Name(0)), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Configs(gameId, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 valid config, got %d", len(configs))
	}
	if configs[0].Region != "eu" || configs[0].Name != "test" {
		t.Errorf("unexpected config: %+v", configs[0])
	}
}
