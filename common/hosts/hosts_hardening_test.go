package hosts

import (
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withSystemHosts points the package at hostsPath as if it were the system
// hosts file, so AddHosts exercises the backup branch.
func withSystemHosts(t *testing.T, hostsPath string) {
	t.Helper()
	origPathFn := pathFn
	pathFn = func() string { return hostsPath }
	t.Cleanup(func() { pathFn = origPathFn })
}

func countOccurrences(s, substr string) int {
	return strings.Count(s, substr)
}

// countLinesContaining counts the lines that contain substr, which is what
// matters for hosts files: formatting (tabs, spaces) is normalized by the
// writer.
func countLinesContaining(s, substr string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, substr) {
			count++
		}
	}
	return count
}

func TestStrippedContentsDropsOwnedLinesAndNormalizes(t *testing.T) {
	contents := []byte("# a user note\r\n127.0.0.1 localhost\r\n127.0.0.1 game.example.com #ageServer\r\n#127.0.0.1 other.example.com #ageServer-preserved\r\n\r\n")
	got, err := StrippedContents(contents)
	if err != nil {
		t.Fatalf("StrippedContents: %v", err)
	}
	want := "# a user note\n127.0.0.1\tlocalhost\n"
	if string(got) != want {
		t.Fatalf("stripped = %q, want %q", got, want)
	}
}

func TestClassifyBackupMatchesWithLineEndings(t *testing.T) {
	backup := []byte("127.0.0.1 localhost\r\n10.1.1.1 user.example.com\r\n")
	current := []byte("127.0.0.1 localhost\r\n10.1.1.1 user.example.com\r\n127.0.0.1 game.example.com #ageServer\r\n")
	state, err := ClassifyBackup(current, backup)
	if err != nil {
		t.Fatalf("ClassifyBackup: %v", err)
	}
	if state != BackupMatches {
		t.Fatalf("state = %s, want %s", state, BackupMatches)
	}
}

func TestClassifyBackupPreEmpty(t *testing.T) {
	current := []byte("127.0.0.1 game.example.com #ageServer\r\n")
	state, err := ClassifyBackup(current, nil)
	if err != nil {
		t.Fatalf("ClassifyBackup: %v", err)
	}
	if state != BackupPreEmpty {
		t.Fatalf("state = %s, want %s", state, BackupPreEmpty)
	}
}

func TestClassifyBackupCurrentLost(t *testing.T) {
	state, err := ClassifyBackup(nil, []byte("127.0.0.1 localhost\n"))
	if err != nil {
		t.Fatalf("ClassifyBackup: %v", err)
	}
	if state != BackupCurrentLost {
		t.Fatalf("state = %s, want %s", state, BackupCurrentLost)
	}
}

func TestClassifyBackupRejectsGarbage11Bytes(t *testing.T) {
	current := []byte("127.0.0.1 localhost\n")
	backup := []byte("#ageServer\n")
	if len(backup) != 11 {
		t.Fatalf("test setup: backup is %d bytes, want 11", len(backup))
	}
	state, err := ClassifyBackup(current, backup)
	if err != nil {
		t.Fatalf("ClassifyBackup: %v", err)
	}
	if state != BackupForeign {
		t.Fatalf("state = %s, want %s: a marker-only file is not a restorable hosts file", state, BackupForeign)
	}
}

func TestClassifyBackupRejectsTruncatedPrefix(t *testing.T) {
	current := []byte("127.0.0.1 localhost\n10.1.1.1 user.example.com\n127.0.0.1 game.example.com #ageServer\n")
	backup := []byte("127.0.0.1 loc")
	state, err := ClassifyBackup(current, backup)
	if err != nil {
		t.Fatalf("ClassifyBackup: %v", err)
	}
	if state != BackupForeign {
		t.Fatalf("state = %s, want %s", state, BackupForeign)
	}
}

func TestAddHostsCreatesBackupWithoutOwnedLines(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	bakPath := filepath.Join(dir, "hosts.bak")
	current := "127.0.0.1 localhost\r\n10.1.1.1 user.example.com\r\n127.0.0.1 stale.example.com #ageServer\r\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	withSystemHosts(t, hostsPath)

	ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", "", "\r\n", false, nil)
	if err != nil || !ok {
		t.Fatalf("AddHosts: ok=%t err=%v", ok, err)
	}
	backup, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if strings.Contains(string(backup), "#ageServer") {
		t.Fatalf("backup contains one of our lines: %q", backup)
	}
	wantBackup := "127.0.0.1\tlocalhost\n10.1.1.1\tuser.example.com\n"
	if string(backup) != wantBackup {
		t.Fatalf("backup = %q, want %q", backup, wantBackup)
	}
}

func TestAddHostsCleansContaminatedLegacyBackup(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	bakPath := filepath.Join(dir, "hosts.bak")
	userLines := "127.0.0.1\tlocalhost\n10.1.1.1\tuser.example.com\n"
	// Both the hosts file and the backup come from an older version that
	// copied our own line into the backup.
	contaminated := userLines + "127.0.0.1\tgame.example.com #ageServer\n"
	if err := os.WriteFile(hostsPath, []byte(contaminated), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, []byte(contaminated), 0644); err != nil {
		t.Fatal(err)
	}
	withSystemHosts(t, hostsPath)

	if ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", "", "\n", false, nil); err != nil || !ok {
		t.Fatalf("AddHosts: ok=%t err=%v", ok, err)
	}
	backup, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != userLines {
		t.Fatalf("backup = %q, want the clean pre-state %q", backup, userLines)
	}
}

func TestAddHostsRotatesForeignBackupInsteadOfFailing(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	bakPath := filepath.Join(dir, "hosts.bak")
	current := "127.0.0.1 localhost\n10.1.1.1 user.example.com\n"
	foreign := "#ageServer\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, []byte(foreign), 0644); err != nil {
		t.Fatal(err)
	}
	withSystemHosts(t, hostsPath)

	ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", "", "\n", false, nil)
	if err != nil || !ok {
		t.Fatalf("AddHosts must not fail because of a foreign backup: ok=%t err=%v", ok, err)
	}
	rotated, err := filepath.Glob(bakPath + ".old-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(rotated) != 1 {
		t.Fatalf("rotated backups = %v, want exactly one", rotated)
	}
	rotatedContent, err := os.ReadFile(rotated[0])
	if err != nil {
		t.Fatalf("read rotated backup: %v", err)
	}
	if string(rotatedContent) != foreign {
		t.Fatalf("rotated backup = %q, want the original %q", rotatedContent, foreign)
	}
	newBackup, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read new backup: %v", err)
	}
	if string(newBackup) != "127.0.0.1\tlocalhost\n10.1.1.1\tuser.example.com\n" {
		t.Fatalf("new backup = %q, want the clean pre-state", newBackup)
	}
	hostsContent, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if countLinesContaining(string(hostsContent), "localhost") != 1 ||
		countLinesContaining(string(hostsContent), "user.example.com") != 1 {
		t.Fatalf("user contents lost or duplicated: %q", hostsContent)
	}
	if !strings.Contains(string(hostsContent), "10.0.0.5") {
		t.Fatalf("new mapping missing: %q", hostsContent)
	}
}

func TestAddHostsAllowsEmptyBackupWhenPreStateEmpty(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	bakPath := filepath.Join(dir, "hosts.bak")
	withSystemHosts(t, hostsPath)

	if ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", "", "\n", false, nil); err != nil || !ok {
		t.Fatalf("first AddHosts: ok=%t err=%v", ok, err)
	}
	info, err := os.Stat(bakPath)
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("backup of an empty pre-state is %d bytes, want 0", info.Size())
	}
	if ok, err := AddHosts(net.ParseIP("10.0.0.6"), "age2", "", "\n", false, nil); err != nil || !ok {
		t.Fatalf("second AddHosts must accept the legitimate empty backup: ok=%t err=%v", ok, err)
	}
}

func TestAddHostsStillRejectsZeroByteBackupWithUserContent(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	bakPath := filepath.Join(dir, "hosts.bak")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 localhost\n10.1.1.1 user.example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	withSystemHosts(t, hostsPath)

	if ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", "", "\n", false, nil); err != nil || !ok {
		t.Fatalf("AddHosts: ok=%t err=%v", ok, err)
	}
	rotated, err := filepath.Glob(bakPath + ".old-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(rotated) != 1 {
		t.Fatalf("empty backup was not rotated: %v", rotated)
	}
	newBackup, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newBackup) != "127.0.0.1\tlocalhost\n10.1.1.1\tuser.example.com\n" {
		t.Fatalf("new backup = %q, want the user content", newBackup)
	}
}

func TestAddHostsCustomPathDoesNotDuplicateContent(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	current := "127.0.0.1 localhost\n10.1.1.1 user.example.com\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	if ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", hostsPath, "\n", false, nil); err != nil || !ok {
		t.Fatalf("AddHosts: ok=%t err=%v", ok, err)
	}
	got, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if occurrences := countLinesContaining(string(got), "localhost"); occurrences != 1 {
		t.Fatalf("localhost appears %d times, want 1: %q", occurrences, got)
	}
	if occurrences := countLinesContaining(string(got), "user.example.com"); occurrences != 1 {
		t.Fatalf("user mapping appears %d times, want 1: %q", occurrences, got)
	}
}

func TestAddHostsCustomPathTwiceIsStable(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	current := "127.0.0.1 localhost\n10.1.1.1 user.example.com\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	if ok, err := AddHosts(net.ParseIP("10.0.0.5"), "age2", hostsPath, "\n", false, nil); err != nil || !ok {
		t.Fatalf("first AddHosts: ok=%t err=%v", ok, err)
	}
	first, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := AddHosts(net.ParseIP("10.0.0.6"), "age2", hostsPath, "\n", false, nil); err != nil || !ok {
		t.Fatalf("second AddHosts: ok=%t err=%v", ok, err)
	}
	second, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimRight(string(first), "\n"), "\n")) != len(strings.Split(strings.TrimRight(string(second), "\n"), "\n")) {
		t.Fatalf("line count changed between runs: %q -> %q", first, second)
	}
	if occurrences := countOccurrences(string(second), "user.example.com"); occurrences != 1 {
		t.Fatalf("user mapping duplicated across runs: %q", second)
	}
	if !strings.Contains(string(second), "10.0.0.6") {
		t.Fatalf("second IP missing: %q", second)
	}
	if strings.Contains(string(second), "10.0.0.5") {
		t.Fatalf("stale first IP remains: %q", second)
	}
}

func TestCommentedAvoidLineHasSpaceBeforeMarker(t *testing.T) {
	// An over-limit line whose marker is glued to the last host: the preserved
	// copy must be reparsable, with a single space before the marker.
	var inner string
	if maxHostsPerLine < math.MaxInt {
		hosts := make([]string, 0, maxHostsPerLine+1)
		for i := 0; i <= maxHostsPerLine; i++ {
			hosts = append(hosts, fmt.Sprintf("h%d.example.com", i))
		}
		inner = "10.1.1.1\t" + strings.Join(hosts, " ")
	} else {
		inner = "10.1.1.1\t" + strings.Repeat("a", maxCharsPerLine) + ".example.com"
	}
	line := inner + "#ageServer"
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(hostsPath, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mappings := HostMappings{}
	rest, err := missingIpMappings(&mappings, f)
	if err != nil {
		t.Fatalf("missingIpMappings: %v", err)
	}
	want := "#" + inner + " #ageServer-preserved"
	for _, l := range rest {
		if l.Preserved() {
			if l.String() != want {
				t.Fatalf("preserved line = %q, want %q", l.String(), want)
			}
			return
		}
	}
	t.Fatalf("no preserved line in %v", rest)
}

func TestMissingIpMappingsDropsStaleOwnedComment(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	current := "127.0.0.1 localhost\n#127.0.0.1 game.example.com #ageServer\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mappings := HostMappings{"game.example.com": net.ParseIP("10.0.0.9")}
	rest, err := missingIpMappings(&mappings, f)
	if err != nil {
		t.Fatalf("missingIpMappings: %v", err)
	}
	for _, l := range rest {
		if strings.Contains(l.String(), "game.example.com") {
			t.Fatalf("stale owned comment kept: %q", l.String())
		}
	}
}

func TestMissingIpMappingsKeepsForeignCommentUntouched(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	current := "127.0.0.1 localhost\n# mi nota\n"
	if err := os.WriteFile(hostsPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mappings := HostMappings{"game.example.com": net.ParseIP("10.0.0.9")}
	rest, err := missingIpMappings(&mappings, f)
	if err != nil {
		t.Fatalf("missingIpMappings: %v", err)
	}
	found := false
	for _, l := range rest {
		if l.String() == "# mi nota" {
			if l.Own() {
				t.Fatalf("foreign comment got an own marking: %q", l.String())
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("foreign comment lost: %v", rest)
	}
}

func TestOverLimitOwnedLinePreservesAllHosts(t *testing.T) {
	if maxHostsPerLine >= 10 {
		t.Skip("the per-line host limit only truncates on Windows")
	}
	userHosts := make([]string, 0, maxHostsPerLine+1)
	for i := 0; i <= maxHostsPerLine; i++ {
		userHosts = append(userHosts, fmt.Sprintf("user%d.example.com", i))
	}
	line := "127.0.0.1\tgame.example.com " + strings.Join(userHosts, " ") + " #ageServer"
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(hostsPath, []byte(line+"\r\n127.0.0.1 localhost\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mappings := HostMappings{"game.example.com": net.ParseIP("10.0.0.9")}
	rest, err := missingIpMappings(&mappings, f)
	if err != nil {
		t.Fatalf("missingIpMappings: %v", err)
	}
	want := "#127.0.0.1\t" + strings.Join(userHosts, " ") + " #ageServer-preserved"
	for _, l := range rest {
		if !l.Preserved() {
			continue
		}
		if l.String() != want {
			t.Fatalf("preserved line = %q, want %q", l.String(), want)
		}
		inner := l.WithoutOwnMarking().Uncommented()
		innerOk, _, reparsed := ParseLine(inner, true)
		if !innerOk || len(reparsed.Hosts()) != len(userHosts) {
			t.Fatalf("preserved %d hosts, want %d (%q)", len(reparsed.Hosts()), len(userHosts), inner)
		}
		for _, host := range reparsed.Hosts() {
			if host == Host("game.example.com") {
				t.Fatalf("preserved copy reactivates our host: %q", inner)
			}
		}
		return
	}
	t.Fatalf("no preserved line in %v", rest)
}

func TestLinePreservedMarkingRoundTrip(t *testing.T) {
	original := Line{ip: net.ParseIP("10.1.1.1"), hosts: []Host{"user.example.com"}, comments: []string{"nota"}}
	ok, commented := original.Commented()
	if !ok {
		t.Fatal("Commented failed")
	}
	commented = commented.WithPreservedMarking()
	if !commented.Preserved() || !commented.Own() {
		t.Fatalf("line is not preserved: %q", commented.String())
	}
	inner := commented.WithoutOwnMarking().Uncommented()
	want := fmt.Sprintf("10.1.1.1\tuser.example.com #nota")
	if inner != want {
		t.Fatalf("uncommented = %q, want %q", inner, want)
	}
}
