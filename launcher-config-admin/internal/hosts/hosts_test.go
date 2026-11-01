package hosts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func setupHostsEnv(t *testing.T) (hostsPath, bakPath string) {
	t.Helper()
	oldWindir := os.Getenv("WINDIR")
	oldLogger := internal.Logger
	internal.Logger = nil
	t.Cleanup(func() {
		os.Setenv("WINDIR", oldWindir)
		internal.Logger = oldLogger
	})
	tmp := t.TempDir()
	os.Setenv("WINDIR", tmp)
	hostsPath = filepath.Join(tmp, "System32", "drivers", "etc", "hosts")
	bakPath = filepath.Join(tmp, "System32", "drivers", "etc", "hosts.bak")
	if err := os.MkdirAll(filepath.Dir(hostsPath), 0755); err != nil {
		t.Fatal(err)
	}
	return
}

func TestRemoveHostsNoFiles(t *testing.T) {
	hostsPath, _ := setupHostsEnv(t)
	// Ensure no files exist
	os.Remove(hostsPath)
	os.Remove(hostsPath + ".bak")
	if err := RemoveHosts(); err != nil {
		t.Fatalf("expected nil when no hosts, got %v", err)
	}
}

func TestRemoveHostsOnlyHostsNoBackup(t *testing.T) {
	hostsPath, _ := setupHostsEnv(t)
	// Create hosts with an owned line
	content := "127.0.0.1 mygame.example.com #ageServer\r\n127.0.0.1 other.com\r\n"
	if err := os.WriteFile(hostsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts failed: %v", err)
	}
	// After removal, owned line should be gone, other line should remain
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s == "" {
		t.Fatal("hosts file should not be empty after removing owned entry")
	}
	if contains(s, "mygame.example.com") {
		t.Fatalf("owned host should be removed, got %q", s)
	}
	if !contains(s, "other.com") {
		t.Fatalf("non-owned host should remain, got %q", s)
	}
}

func TestRemoveHostsWithBackupNewerRestoresBackup(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	// Create backup with known content
	bakContent := "127.0.0.1 original.com\r\n"
	if err := os.WriteFile(bakPath, []byte(bakContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Ensure backup is newer than hosts
	time.Sleep(1100 * time.Millisecond)
	hostsContent := "127.0.0.1 owned.com #ageServer\r\n127.0.0.1 other.com\r\n"
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Set backup mtime to be after hosts (already is, but ensure)
	now := time.Now()
	os.Chtimes(bakPath, now, now)
	os.Chtimes(hostsPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts with backup newer failed: %v", err)
	}
	// After restoreBackup, hosts should be replaced with backup content, and backup removed
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed after restore, err %v", err)
	}
	data, _ := os.ReadFile(hostsPath)
	if string(data) != bakContent {
		t.Fatalf("hosts after restore = %q, want %q", string(data), bakContent)
	}
}

func TestRemoveHostsWithBackupOlderDoesInPlace(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	hostsContent := "127.0.0.1 owned2.com #ageServer\r\n127.0.0.1 keep.com\r\n"
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	bakContent := "127.0.0.1 original2.com\r\n"
	if err := os.WriteFile(bakPath, []byte(bakContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Make backup older than hosts
	now := time.Now()
	os.Chtimes(hostsPath, now, now)
	os.Chtimes(bakPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts with backup older failed: %v", err)
	}
	// Backup should be removed, and hosts should have owned line removed via in-place
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed after in-place, err %v", err)
	}
	data, _ := os.ReadFile(hostsPath)
	if contains(string(data), "owned2.com") {
		t.Fatalf("owned2 should be removed, got %q", string(data))
	}
}

func TestRemoveHostsBackupWithNoMainCreatesMain(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	os.Remove(hostsPath)
	bakContent := "127.0.0.1 restored.com\r\n"
	if err := os.WriteFile(bakPath, []byte(bakContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts with only backup failed: %v", err)
	}
	// Hosts should now exist with backup content, backup removed
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatalf("hosts should exist after restore, err %v", err)
	}
	if string(data) != bakContent {
		t.Fatalf("hosts = %q want %q", string(data), bakContent)
	}
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatal("backup should be removed")
	}
}

func TestFlushDnsDoesNotPanic(t *testing.T) {
	_, _ = setupHostsEnv(t)
	internal.Logger = nil
	// Test with SetUp nil
	internal.SetUp = nil
	res := FlushDns()
	if res == nil {
		t.Fatal("FlushDns should return result")
	}
	// Test with SetUp true
	v := true
	internal.SetUp = &v
	res = FlushDns()
	if res == nil {
		t.Fatal("FlushDns setup true should return")
	}
	// Test with SetUp false
	v = false
	internal.SetUp = &v
	res = FlushDns()
	if res == nil {
		t.Fatal("FlushDns setup false should return")
	}
	// Test with logger set
	dir := t.TempDir()
	if err := internal.Initialize(dir); err != nil {
		t.Fatal(err)
	}
	res = FlushDns()
	if res == nil {
		t.Fatal("FlushDns with logger should return")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
