package hosts

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/luskaner/ageServer/common/hosts/text"
)

// StrippedContents returns the decoded contents without any AgeServer line
// (active or commented with either marker), with line endings normalized to
// "\n" and without trailing empty lines. It is the base of every backup
// validation: the backup of a setup run must be exactly the state that
// preceded it, with nothing of ours in it.
func StrippedContents(contents []byte) ([]byte, error) {
	decoded, _, err := text.Decode(contents)
	if err != nil {
		return nil, fmt.Errorf("cannot decode hosts contents: %w", err)
	}
	lines := strings.Split(decoded, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			// Blank lines are formatting only; dropping them makes the
			// comparison independent of how many separators a run wrote.
			continue
		}
		ok, _, parsed := ParseLine(line, true)
		if !ok {
			kept = append(kept, line)
			continue
		}
		if parsed.Own() {
			continue
		}
		// Render parseable lines the same way the writer does, so a file
		// rewritten by a previous setup still matches its own backup.
		kept = append(kept, parsed.String())
	}
	if len(kept) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(kept, "\n") + "\n"), nil
}

// BackupState classifies an existing hosts.bak against the current hosts file.
type BackupState int

const (
	// BackupMatches means the backup holds the state that preceded the current
	// AgeServer mappings.
	BackupMatches BackupState = iota
	// BackupPreEmpty means both the current and the backup states had no user
	// content (the backup is a legitimate 0-byte file).
	BackupPreEmpty
	// BackupCurrentLost means the current file lost its user content (for
	// example it was truncated to 0 bytes) while the backup still holds a
	// plausible hosts file. The backup is the only healthy source.
	BackupCurrentLost
	// BackupForeign means the backup cannot be trusted and must be rotated.
	BackupForeign
)

// String makes test failures readable.
func (s BackupState) String() string {
	switch s {
	case BackupMatches:
		return "matches"
	case BackupPreEmpty:
		return "pre-empty"
	case BackupCurrentLost:
		return "current-lost"
	default:
		return "foreign"
	}
}

// ClassifyBackup decides whether an existing hosts.bak can be used to restore
// the state that preceded the current AgeServer mappings. It performs no I/O:
// it receives already read contents.
func ClassifyBackup(current, backup []byte) (BackupState, error) {
	strippedCurrent, err := StrippedContents(current)
	if err != nil {
		return BackupForeign, fmt.Errorf("current hosts file: %w", err)
	}
	strippedBackup, err := StrippedContents(backup)
	if err != nil {
		return BackupForeign, fmt.Errorf("hosts backup: %w", err)
	}
	if len(bytes.TrimSpace(strippedCurrent)) == 0 && len(bytes.TrimSpace(strippedBackup)) == 0 {
		return BackupPreEmpty, nil
	}
	if bytes.Equal(strippedCurrent, strippedBackup) {
		return BackupMatches, nil
	}
	if len(bytes.TrimSpace(strippedCurrent)) != 0 {
		return BackupForeign, nil
	}
	if backupLooksLikeHosts(backup) {
		return BackupCurrentLost, nil
	}
	return BackupForeign, nil
}

// containsOwnedLines reports whether any line carries an AgeServer marker.
func containsOwnedLines(contents []byte) (bool, error) {
	decoded, _, err := text.Decode(contents)
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(strings.NewReader(decoded))
	for scanner.Scan() {
		ok, _, parsed := ParseLine(scanner.Text(), true)
		if ok && parsed.Own() {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// backupLooksLikeHosts reports whether the backup decodes to a plain hosts file
// with no AgeServer lines and at least one usable mapping. Requiring a real
// mapping is what keeps garbage such as "#ageServer\n" from being restored.
func backupLooksLikeHosts(backup []byte) bool {
	decoded, _, err := text.Decode(backup)
	if err != nil {
		return false
	}
	hasMapping := false
	scanner := bufio.NewScanner(strings.NewReader(decoded))
	for scanner.Scan() {
		ok, _, parsed := ParseLine(scanner.Text(), true)
		if !ok {
			continue
		}
		if parsed.Own() {
			return false
		}
		if !parsed.OnlyComments() && parsed.IP() != nil && len(parsed.Hosts()) > 0 {
			hasMapping = true
		}
	}
	if scanner.Err() != nil {
		return false
	}
	return hasMapping
}
