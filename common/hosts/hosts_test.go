package hosts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLineOnlyCommentsWithHosts(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"a.com"}}
	if l.OnlyComments() {
		t.Error("Line with ip and hosts should not be OnlyComments")
	}
}

func TestLineOwnNoComments(t *testing.T) {
	l := Line{}
	if l.Own() {
		t.Error("Empty Line should not be Own")
	}
}

func TestLineOwnWrongComment(t *testing.T) {
	l := Line{comments: []string{"other"}}
	if l.Own() {
		t.Error("Line without marking should not be Own")
	}
}

func TestLineStringWithComments(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	hosts := []Host{"a.com"}
	l := Line{ip: ip, hosts: hosts, comments: []string{"note"}}
	s := l.String()
	if !strings.Contains(s, "10.0.0.1") || !strings.Contains(s, "a.com") || !strings.Contains(s, "#note") {
		t.Errorf("String() = %q", s)
	}
}

func TestLineStringNoIP(t *testing.T) {
	l := Line{comments: []string{"hello", "world"}}
	s := l.String()
	if !strings.Contains(s, "#hello#world") {
		t.Errorf("String() = %q", s)
	}
}

func TestLineUncommentedWithHosts(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"test.com"}}
	result := l.Uncommented()
	if result == "" || !strings.Contains(result, "1.2.3.4") {
		t.Errorf("Uncommented() = %q", result)
	}
}

func TestHostMappingsGetMissing(t *testing.T) {
	m := make(HostMappings)
	_, ok := m.Get("nonexistent.com")
	if ok {
		t.Error("Get on empty map should return false")
	}
}

func TestCommentLineStrValid(t *testing.T) {
	ok, l := commentLineStr("127.0.0.1 example.com")
	if !ok {
		t.Fatal("commentLineStr should succeed")
	}
	if !l.hasOwnMarking() {
		t.Error("should have own marking")
	}
}

func TestCommentLineStrEmpty(t *testing.T) {
	ok, l := commentLineStr("")
	if !ok {
		t.Fatal("commentLineStr of empty should succeed (comment-only)")
	}
	if !l.OnlyComments() {
		t.Error("empty + comment marker should be OnlyComments")
	}
}

func TestCommentLineStrInvalid(t *testing.T) {
	// "not-valid" is a single token, no IP → becomes comment-only with marker
	ok, l := commentLineStr("not-valid")
	if !ok {
		t.Fatal("commentLineStr should succeed")
	}
	if !l.OnlyComments() {
		t.Error("invalid line + comment marker should be OnlyComments")
	}
}

func TestGetAllLines(t *testing.T) {
	content := "127.0.0.1 localhost\n10.0.0.1 example.com\n# comment\n"
	dir := t.TempDir()
	f := filepath.Join(dir, "hosts")
	os.WriteFile(f, []byte(content), 0644)
	file, err := os.Open(f)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	err, _, lines := GetAllLines(file)
	if err != nil {
		t.Fatalf("GetAllLines: %v", err)
	}
	if len(lines) < 2 {
		t.Errorf("expected >= 2 lines, got %d", len(lines))
	}
}

func TestGetAllLinesDeduplicatesHosts(t *testing.T) {
	content := "10.0.0.1 dup.com\n10.0.0.2 dup.com\n"
	dir := t.TempDir()
	f := filepath.Join(dir, "hosts")
	os.WriteFile(f, []byte(content), 0644)
	file, err := os.Open(f)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, _, lines := GetAllLines(file)
	for _, l := range lines {
		for _, h := range l.Hosts() {
			if h == "dup.com" && len(l.Hosts()) > 1 {
				t.Error("duplicate host should be removed")
			}
		}
	}
}

func TestCommentHostAlwaysFalseOnWindows(t *testing.T) {
	if commentHost("anything") {
		t.Error("commentHost should return false on Windows")
	}
}

func TestPathNotEmpty(t *testing.T) {
	if Path() == "" {
		t.Error("Path() should not be empty")
	}
}

func TestCreateTemp(t *testing.T) {
	lock, err := CreateTemp()
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if lock == nil || lock.File == nil {
		t.Fatal("lock.File should not be nil")
	}
	defer func() {
		name := lock.File.Name()
		_ = lock.Unlock()
		_ = os.Remove(name)
	}()
}

func TestOpenMainAndGetAllLines(t *testing.T) {
	dir := t.TempDir()
	// Create a temp hosts file and test openHostsFile directly via AddHosts
	content := "127.0.0.1 localhost\n"
	fpath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _, lines := GetAllLines(f)
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

func TestMissingIpMappings(t *testing.T) {
	content := "127.0.0.1 localhost\n10.0.0.1 old.example.com\n"
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mappings := HostMappings{"new.example.com": net.ParseIP("10.0.0.2")}
	rest, err := missingIpMappings(&mappings, f)
	if err != nil {
		t.Fatalf("missingIpMappings: %v", err)
	}
	// old.example.com not in mappings, should be kept
	if len(rest) == 0 {
		t.Error("expected restLines")
	}
}

func TestAddHostsTempFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	// Start with empty file
	if err := os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP("10.0.0.5")
	ok, err := AddHosts(ip, "age2", fpath, "\r\n", false, nil)
	if err != nil {
		t.Fatalf("AddHosts: %v", err)
	}
	if !ok {
		t.Error("AddHosts should return ok=true")
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "10.0.0.5") {
		t.Errorf("hosts file should contain new IP, got %q", string(data))
	}
}

func TestAddHostsAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	ip := net.ParseIP("10.0.0.5")
	// First add
	if _, err := AddHosts(ip, "age2", fpath, "\n", false, nil); err != nil {
		t.Fatalf("first AddHosts: %v", err)
	}
	// Second add same IP should be ok (already present)
	ok, err := AddHosts(ip, "age2", fpath, "\n", false, nil)
	if err != nil {
		t.Fatalf("second AddHosts: %v", err)
	}
	if !ok {
		t.Error("AddHosts should be ok when already present")
	}
}

func TestUpdateHostsViaAddHosts(t *testing.T) {
	// UpdateHosts is exercised via AddHosts; this smoke test ensures it doesn't panic
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP("10.0.0.99")
	ok, err := AddHosts(ip, "age1", fpath, "\n", false, nil)
	if err != nil {
		t.Fatalf("AddHosts: %v", err)
	}
	if !ok {
		t.Error("expected ok")
	}
}

func TestAddHostsOpenError(t *testing.T) {
	dir := t.TempDir()
	// Pass a directory as file path, OpenFile will fail with "is a directory"
	_, err := AddHosts(net.ParseIP("1.2.3.4"), "age2", dir, "\n", false, nil)
	if err == nil {
		t.Error("expected error when hostFilePath is a directory")
	}
}

func TestAddHostsNotExistParent(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "nonexistent", "hosts")
	_, err := AddHosts(net.ParseIP("1.2.3.4"), "age2", fpath, "\n", false, nil)
	if err == nil {
		t.Error("expected error when parent dir does not exist")
	}
}

func TestAddHostsDecodeError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	// Single 0xFF causes Decode to fail with unsupported encoding
	if err := os.WriteFile(fpath, []byte{0xFF}, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := AddHosts(net.ParseIP("1.2.3.4"), "age2", fpath, "\n", false, nil)
	if err == nil {
		t.Error("expected error when hosts file has invalid encoding")
	}
}

func TestAddHostsLineEndingDefault(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644)
	ip := net.ParseIP("1.2.3.4")
	ok, err := AddHosts(ip, "age2", fpath, "", false, nil)
	if err != nil || !ok {
		t.Fatalf("AddHosts with empty lineEnding should use default, err=%v ok=%v", err, ok)
	}
}

func TestOpenLockedBackupAndMain(t *testing.T) {
	// These functions use Path() which is system hosts path, but we test they don't panic
	// and return error when file doesn't exist or cannot be locked
	_, err := OpenLockedBackup(os.O_RDONLY)
	// May succeed or fail depending on system, just ensure no panic
	_ = err
	_, err = OpenLockedMain(os.O_RDONLY)
	_ = err
	_, err = OpenMain()
	_ = err
}

func TestCreateTempError(t *testing.T) {
	// CreateTemp should succeed in normal case, just verify no panic
	lock, err := CreateTemp()
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if lock != nil && lock.File != nil {
		name := lock.File.Name()
		_ = lock.Unlock()
		_ = os.Remove(name)
	}
}

func TestGetAllLinesDecodeError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte{0xFF}, 0644)
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err, _, _ = GetAllLines(f)
	if err == nil {
		t.Error("GetAllLines should error on invalid encoding")
	}
}

func TestDecodeAndGetScannerError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte{0xFF}, 0644)
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err, _, _ = decodeAndGetScanner(f)
	if err == nil {
		t.Error("decodeAndGetScanner should error on invalid encoding")
	}
}

func TestMissingIpMappingsDecodeError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte{0xFF}, 0644)
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := HostMappings{}
	_, err = missingIpMappings(&m, f)
	if err == nil {
		t.Error("missingIpMappings should error on invalid encoding")
	}
}

func TestUpdateHostsError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644)
	lock, err := openLockedHostsFile(fpath, os.O_RDWR)
	if err != nil {
		t.Fatalf("openLocked: %v", err)
	}
	// Don't defer Unlock - UpdateHosts will close on error via its own defer
	err = UpdateHosts(lock, func(f *os.File) error { return os.ErrInvalid }, nil)
	if err == nil {
		t.Error("UpdateHosts should propagate updater error")
	}
	// After error, lock is already closed, don't call Unlock again
}

func TestAddHostsSystemHosts(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "hosts")
	_ = os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644)
	origPathFn := pathFn
	pathFn = func() string { return fpath }
	defer func() { pathFn = origPathFn }()
	// Ensure backup doesn't exist
	_ = os.Remove(fpath + ".bak")
	ip := net.ParseIP("10.99.0.1")
	ok, err := AddHosts(ip, "age2", "", "\n", false, nil)
	if err != nil {
		t.Fatalf("AddHosts systemHosts: %v", err)
	}
	if !ok {
		t.Error("expected ok")
	}
	// Backup should have been created
	if _, err := os.Stat(fpath + ".bak"); err != nil {
		t.Errorf("backup not created: %v", err)
	}
	_ = os.Remove(fpath + ".bak")
}

func TestLineWithoutOwnMarkingNoMarking(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"a.com"}, comments: []string{"note"}}
	// No marking, should return same line
	got := l.WithoutOwnMarking()
	if got.String() != l.String() {
		t.Errorf("WithoutOwnMarking without marking should return same, got %q want %q", got.String(), l.String())
	}
}

func TestLineWithoutOwnMarkingWithMarking(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"a.com"}}
	l = l.WithOwnMarking()
	got := l.WithoutOwnMarking()
	if got.Own() {
		t.Error("WithoutOwnMarking should remove marking")
	}
}

func TestLineWithOwnMarkingAlreadyMarked(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"a.com"}}
	l = l.WithOwnMarking()
	origStr := l.String()
	got := l.WithOwnMarking()
	if got.String() != origStr {
		t.Errorf("WithOwnMarking already marked should return same, got %q want %q", got.String(), origStr)
	}
}

func TestLineWithOwnMarkingNotMarked(t *testing.T) {
	l := Line{ip: net.ParseIP("1.2.3.4"), hosts: []Host{"a.com"}}
	got := l.WithOwnMarking()
	if !got.Own() {
		t.Error("WithOwnMarking should add marking")
	}
}
