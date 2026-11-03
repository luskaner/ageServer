//go:build windows

package hosts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	commonHosts "github.com/luskaner/ageServer/common/hosts"
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

func TestRemoveHostsWithMatchingBackupRestoresBackup(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	bakContent := "127.0.0.1 original.com\r\n"
	if err := os.WriteFile(bakPath, []byte(bakContent), 0644); err != nil {
		t.Fatal(err)
	}
	// The current file is the pre-setup state plus our own mappings, which is
	// exactly what a validated backup describes.
	hostsContent := "127.0.0.1 owned.com #ageServer\r\n" + bakContent
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	os.Chtimes(bakPath, now, now)
	os.Chtimes(hostsPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts with matching backup failed: %v", err)
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

func TestStaleAgeServerBackupCanBeReusedAndReverted(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	backup := "127.0.0.1\tlocalhost\r\n"
	mappings := commonHosts.Mappings("age2", net.ParseIP("127.0.0.1"), false)
	var oldHost string
	for host := range mappings {
		oldHost = host.String()
		break
	}
	if oldHost == "" {
		t.Fatal("age2 mappings should not be empty")
	}
	current := "127.0.0.1\t" + oldHost + " #ageServer\r\n" + backup
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, []byte(backup), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(bakPath, now.Add(-2*time.Second), now.Add(-2*time.Second)); err != nil {
		t.Fatal(err)
	}

	ok, err := commonHosts.AddHosts(net.ParseIP("172.30.224.1"), "age2", "", "", false, nil)
	if err != nil || !ok {
		t.Fatalf("AddHosts with a stale backup: ok=%t err=%v", ok, err)
	}
	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts after retry: %v", err)
	}

	got, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), oldHost) || strings.Contains(string(got), "#ageServer") {
		t.Fatalf("stale AgeServer mappings remain after teardown: %q", got)
	}
	if string(got) != backup {
		t.Fatalf("hosts after teardown = %q, want original %q", got, backup)
	}
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed after teardown, err=%v", err)
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

// gameHost returns one of the hosts mapped for the given game.
func gameHost(t *testing.T, gameId string) string {
	t.Helper()
	for host := range commonHosts.Mappings(gameId, net.ParseIP("127.0.0.1"), false) {
		return host.String()
	}
	t.Fatalf("no mappings for game %q", gameId)
	return ""
}

// A legacy owned comment must never become an active mapping on revert.
func TestRevertDoesNotActivateLegacyOwnedComment(t *testing.T) {
	hostsPath, _ := setupHostsEnv(t)
	host := gameHost(t, "age2")
	content := "127.0.0.1 localhost\r\n#127.0.0.1 " + host + " #ageServer\r\n"
	if err := os.WriteFile(hostsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if ok, err := commonHosts.AddHosts(net.ParseIP("10.1.1.1"), "age2", "", "", false, nil); err != nil || !ok {
		t.Fatalf("first AddHosts: ok=%t err=%v", ok, err)
	}
	if ok, err := commonHosts.AddHosts(net.ParseIP("10.1.1.2"), "age2", "", "", false, nil); err != nil || !ok {
		t.Fatalf("second AddHosts: ok=%t err=%v", ok, err)
	}
	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}

	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), host) {
		t.Fatalf("legacy owned comment became an active mapping: %q", data)
	}
	if strings.Contains(string(data), "#ageServer") {
		t.Fatalf("AgeServer markers remain after revert: %q", data)
	}
	if string(data) != "127.0.0.1\tlocalhost\n" {
		t.Fatalf("hosts after revert = %q, want the original localhost line", data)
	}
}

// The same protection applies to the in-place revert path, which has no backup.
func TestRevertInPlaceDropsOrphanLegacyOwnedComment(t *testing.T) {
	hostsPath, _ := setupHostsEnv(t)
	host := gameHost(t, "age2")
	content := "127.0.0.1 localhost\r\n#127.0.0.1 " + host + " #ageServer\r\n"
	if err := os.WriteFile(hostsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), host) {
		t.Fatalf("orphan legacy comment became an active mapping: %q", data)
	}
	if !strings.Contains(string(data), "127.0.0.1 localhost") {
		t.Fatalf("localhost mapping lost: %q", data)
	}
}

// A user line that setup had to comment is restored exactly once, with a
// single space before its marker.
func TestRevertKeepsUserLineCommentedBySetup(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	host := gameHost(t, "age2")
	original := "127.0.0.1\tuser.example.com " + host
	if err := os.WriteFile(hostsPath, []byte(original+"\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if ok, err := commonHosts.AddHosts(net.ParseIP("10.1.1.1"), "age2", "", "", false, nil); err != nil || !ok {
		t.Fatalf("AddHosts: ok=%t err=%v", ok, err)
	}
	// Force the in-place revert: it must use our preserved copy.
	if err := os.Remove(bakPath); err != nil {
		t.Fatal(err)
	}
	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}

	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimRight(string(data), "\r\n")
	if got != original {
		t.Fatalf("hosts after in-place revert = %q, want %q", got, original)
	}
	if strings.Count(got, "user.example.com") != 1 {
		t.Fatalf("user line duplicated: %q", got)
	}
}

// A foreign backup must never replace the whole file.
func TestRemoveHostsRefusesForeignBackupAndFallsBackInPlace(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	host := gameHost(t, "age2")
	current := "127.0.0.1 localhost\r\n127.0.0.1 " + host + " #ageServer\r\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, []byte("#ageServer\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	os.Chtimes(bakPath, now, now)
	os.Chtimes(hostsPath, now.Add(-2*time.Second), now.Add(-2*time.Second))

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == len("#ageServer\r\n") && strings.Contains(string(data), "#ageServer") {
		t.Fatalf("foreign backup replaced the hosts file: %q", data)
	}
	if !strings.Contains(string(data), "127.0.0.1 localhost") {
		t.Fatalf("user mappings lost: %q", data)
	}
	if strings.Contains(string(data), host) {
		t.Fatalf("owned mapping not removed: %q", data)
	}
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed after an in-place revert, err=%v", err)
	}
}

// A hosts file that lost its contents is recovered from a plausible backup.
func TestRemoveHostsRecoversFromEmptyHosts(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	if err := os.WriteFile(hostsPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	bakContent := "127.0.0.1 localhost\r\n"
	if err := os.WriteFile(bakPath, []byte(bakContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != bakContent {
		t.Fatalf("hosts after recovery = %q, want %q", data, bakContent)
	}
}

// An empty pre-state is a legitimate backup, not a corrupt one.
func TestRemoveHostsEmptyPreStateRestoresEmpty(t *testing.T) {
	hostsPath, bakPath := setupHostsEnv(t)
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 owned.com #ageServer\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveHosts(); err != nil {
		t.Fatalf("RemoveHosts: %v", err)
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("hosts after revert = %q, want an empty file", data)
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
