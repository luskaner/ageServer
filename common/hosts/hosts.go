package hosts

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/fileLock"
	"github.com/luskaner/ageServer/common/hosts/text"
	commonLogger "github.com/luskaner/ageServer/common/logger"
)

var pathFn = Path

func CreateTemp() (lock *fileLock.Lock, err error) {
	var f *os.File
	//goland:noinspection ALL
	f, err = os.CreateTemp("", common.Name+"_host_*.txt")
	if err != nil {
		return
	}
	lock = &fileLock.Lock{}
	if err = lock.Lock(f); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	return
}
func OpenLockedBackup(flags int) (lock *fileLock.Lock, err error) {
	return openLockedHostsFile(filepath.Join(filepath.Dir(pathFn()), "hosts.bak"), flags)
}

func OpenLockedMain(flags int) (lock *fileLock.Lock, err error) {
	return openLockedHostsFile(pathFn(), flags)
}

func OpenMain() (f *os.File, err error) {
	return openHostsFile(pathFn(), os.O_RDONLY)
}

func decodeAndGetScanner(f *os.File) (err error, encType int, scanner *bufio.Scanner) {
	var buf []byte
	buf, err = io.ReadAll(f)
	var str string
	str, encType, err = text.Decode(buf)
	if err != nil {
		return
	}
	scanner = bufio.NewScanner(strings.NewReader(str))
	return
}

func GetAllLines(f *os.File) (err error, encType int, lines []Line) {
	var scanner *bufio.Scanner
	err, encType, scanner = decodeAndGetScanner(f)
	if err != nil {
		return
	}
	addedHosts := mapset.NewThreadUnsafeSet[Host]()
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		ok, _, parsedLine := ParseLine(line, true)
		if !ok || parsedLine.OnlyComments() {
			continue
		}
		var finalHosts []Host
		for _, host := range parsedLine.Hosts() {
			if !addedHosts.ContainsOne(host) {
				finalHosts = append(finalHosts, host)
				addedHosts.Add(host)
			}
		}
		if len(finalHosts) > 0 {
			lines = append(lines, Line{ip: parsedLine.ip, hosts: finalHosts}.WithOwnMarking())
		}
	}
	err = scanner.Err()
	return
}

func commentLineStr(line string) (ok bool, l Line) {
	line = string(commentMarker) + line
	ok, _, l = ParseLine(line, true)
	if !ok {
		return
	}
	if ok, l = l.Commented(); ok {
		l = l.WithOwnMarking()
	}
	return
}

// normalizeOwnMarkerSpacing makes sure the own marker is separated from the
// rest of the line by exactly one space, so the commented copy that a revert
// uncomments has a stable, reparsable shape.
func normalizeOwnMarkerSpacing(line string) string {
	line = strings.TrimRight(line, " \t")
	marker := string(commentMarker) + marking
	if !strings.HasSuffix(line, marker) {
		return line
	}
	prefix := strings.TrimSuffix(line, marker)
	if prefix == "" {
		return line
	}
	return strings.TrimRight(prefix, " \t") + " " + marker
}

// verifyPreservedLine checks that a commented copy round-trips back to a
// usable mapping when a revert uncomments it.
func verifyPreservedLine(l Line) bool {
	if !l.OnlyComments() || !l.Preserved() {
		return false
	}
	inner := l.WithoutOwnMarking().Uncommented()
	ok, _, parsed := ParseLine(inner, true)
	return ok && !parsed.OnlyComments() && parsed.IP() != nil && len(parsed.Hosts()) > 0
}

// normalizeOwnedComment decides what to do with a comment-only line that
// carries an AgeServer marker.
//
// Preserved copies (written by this version) are kept untouched: a revert
// uncomments them. Legacy owned comments (the bare marker) are never revived:
// if they are a stale copy of a host this run is about to map they are dropped,
// and otherwise they are kept as a plain comment without any marker.
func normalizeOwnedComment(l Line, mappings *HostMappings) (Line, bool) {
	if !l.Own() || l.Preserved() {
		return l, false
	}
	inner := l.WithoutOwnMarking().Uncommented()
	ok, _, parsed := ParseLine(inner, true)
	if !ok || parsed.OnlyComments() || parsed.IP() == nil || len(parsed.Hosts()) == 0 {
		cleaned := l.WithoutOwnMarking()
		if len(cleaned.comments) == 0 {
			return Line{}, true
		}
		return cleaned, false
	}
	for _, host := range parsed.Hosts() {
		if _, inMappings := mappings.Get(host); inMappings {
			// Stale copy of a mapping this run writes again.
			return Line{}, true
		}
	}
	return l.WithoutOwnMarking(), false
}

func missingIpMappings(mappings *HostMappings, hostFile *os.File) (restLines []Line, err error) {
	var scanner *bufio.Scanner
	err, _, scanner = decodeAndGetScanner(hostFile)
	if err != nil {
		return
	}
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		// Blank lines carry no information and would accumulate between runs
		// because the generated block starts with its own separator.
		if strings.TrimSpace(line) == "" {
			continue
		}
		ok, overLimit, lineParsed := ParseLine(line, false)
		if !ok {
			ok, lineParsed = commentLineStr(line)
			if !ok {
				continue
			}
		}
		if lineParsed.OnlyComments() {
			normalized, drop := normalizeOwnedComment(lineParsed, mappings)
			if drop {
				continue
			}
			restLines = append(restLines, normalized)
			continue
		}
		lineIp := lineParsed.IP()
		hosts := lineParsed.Hosts()
		commentsBefore := lineParsed.comments
		// Parse the line again without the platform limits so the preserved
		// copy keeps every host, including those past maxHostsPerLine.
		fullHosts := hosts
		if fullOk, _, fullParsed := ParseLine(line, true); fullOk {
			fullHosts = fullParsed.Hosts()
			if len(fullParsed.comments) > 0 {
				commentsBefore = fullParsed.comments
			}
		}
		indexesToAvoid := mapset.NewThreadUnsafeSet[int]()
		indexesToRemove := mapset.NewThreadUnsafeSet[int]()
		removedOwnedHostMapping := false
		for i, lineHost := range hosts {
			if ip, ok := mappings.Get(lineHost); ok {
				if ip.Equal(lineIp) {
					mappings.Delete(lineHost)
				} else if lineParsed.Own() {
					indexesToRemove.Add(i)
					removedOwnedHostMapping = true
				} else {
					indexesToAvoid.Add(i)
				}
			}
		}
		var keptHosts []Host
		var removedHosts bool
		for i := range hosts {
			if indexesToRemove.ContainsOne(i) || indexesToAvoid.ContainsOne(i) || commentHost(hosts[i]) {
				removedHosts = true
			} else {
				keptHosts = append(keptHosts, hosts[i])
			}
		}
		if overLimit {
			removedHosts = true
		}
		if removedHosts {
			if removedOwnedHostMapping {
				preservedHosts := make([]Host, 0, len(fullHosts))
				for _, host := range fullHosts {
					if ip, ok := mappings.Get(host); ok && !ip.Equal(lineIp) && lineParsed.Own() {
						continue
					}
					preservedHosts = append(preservedHosts, host)
				}
				if len(preservedHosts) > 0 {
					preservedLine := Line{ip: lineIp, hosts: preservedHosts, comments: commentsBefore}
					if nlOk, nl := preservedLine.Commented(); nlOk {
						nl = nl.WithPreservedMarking()
						if verifyPreservedLine(nl) {
							restLines = append(restLines, nl)
						} else {
							commonLogger.Printf(
								"hosts: cannot preserve mapping %q without corrupting it; keeping it as a plain comment: %q\n",
								lineIp.String(),
								nl.String(),
							)
							restLines = append(restLines, Line{comments: nl.WithoutOwnMarking().comments})
						}
					}
				}
			} else if nlOk, nl := commentLineStr(normalizeOwnMarkerSpacing(line)); nlOk {
				restLines = append(restLines, nl.WithPreservedMarking())
			}
		}
		if len(keptHosts) > 0 {
			nl := Line{ip: lineIp, hosts: keptHosts}
			if removedHosts {
				nl = nl.WithOwnMarking()
			}
			restLines = append(restLines, nl)
		}
	}
	err = scanner.Err()
	return
}

func openHostsFile(hostFilePath string, flag int) (f *os.File, err error) {
	return os.OpenFile(
		hostFilePath,
		flag,
		0644,
	)
}

func openLockedHostsFile(hostFilePath string, flag int) (lock *fileLock.Lock, err error) {
	var f *os.File
	//goland:noinspection ALL
	f, err = openHostsFile(hostFilePath, flag)
	if err == nil {
		lock = &fileLock.Lock{}
		if err = lock.Lock(f); err != nil {
			_ = f.Close()
			f = nil
			err = fmt.Errorf("failed to lock hosts file %q: %w", hostFilePath, err)
		}
	} else {
		err = fmt.Errorf("failed to open hosts file %q: %w", hostFilePath, err)
	}
	return
}

func UpdateHosts(hostsLock *fileLock.Lock, updater func(file *os.File) error, flushFn func() (result *exec.Result)) error {
	closed := false
	var tmpLock *fileLock.Lock = nil

	closeHostsFile := func() {
		_ = hostsLock.File.Sync()
		_ = hostsLock.Unlock()
		closed = true
	}

	removeTmpFile := func() {
		filePath := tmpLock.File.Name()
		_ = tmpLock.Unlock()
		_ = os.Remove(filePath)
		tmpLock = nil
	}

	defer func() {
		if !closed {
			closeHostsFile()
		}
		if tmpLock != nil {
			removeTmpFile()
		}
	}()
	var err error
	tmpLock, err = CreateTemp()
	if err != nil {
		return fmt.Errorf("failed to create temp file for hosts update: %w", err)
	}

	_, err = io.Copy(tmpLock.File, hostsLock.File)

	if err != nil {
		return fmt.Errorf("failed to copy hosts file to temp file: %w", err)
	}

	if err = updater(tmpLock.File); err == nil {
		// Truncate temp file to the actual written size before copying back.
		// Otherwise, if the updater wrote fewer bytes than the original content
		// (e.g. stale owned mappings removed), the tail of the old temp file
		// survives and gets copied back as stale active mappings.
		pos, seekErr := tmpLock.File.Seek(0, io.SeekCurrent)
		if seekErr != nil {
			return fmt.Errorf("failed to get temp hosts file position: %w", seekErr)
		}
		if err = tmpLock.File.Truncate(pos); err != nil {
			return fmt.Errorf("failed to truncate temp hosts file: %w", err)
		}

		_, err = tmpLock.File.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek temp hosts file: %w", err)
		}
		err = hostsLock.File.Truncate(0)
		if err != nil {
			return fmt.Errorf("failed to truncate hosts file %q: %w", hostsLock.File.Name(), err)
		}

		_, err = hostsLock.File.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek hosts file %q: %w", hostsLock.File.Name(), err)
		}

		_, err = io.Copy(hostsLock.File, tmpLock.File)
		if err != nil {
			return fmt.Errorf("failed to copy temp file back to hosts file %q: %w", hostsLock.File.Name(), err)
		}
		removeTmpFile()
		closeHostsFile()
		if flushFn != nil {
			_ = flushFn()
		}
		return nil
	}

	return err
}

func readHostsContents(file *os.File) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return contents, nil
}

func hostsBackupPath() string {
	return filepath.Join(filepath.Dir(pathFn()), "hosts.bak")
}

func nextRotatedBackupPath(bakPath string) string {
	base := bakPath + ".old-" + strconv.FormatInt(time.Now().Unix(), 10)
	rotated := base
	for i := 1; ; i++ {
		if _, err := os.Stat(rotated); errors.Is(err, os.ErrNotExist) {
			return rotated
		}
		rotated = base + "-" + strconv.Itoa(i)
	}
}

// rotateHostsBackup never destroys an unrecognized backup: it is renamed so it
// stays available for inspection, and setup continues.
func rotateHostsBackup(bakPath string) (string, error) {
	rotated := nextRotatedBackupPath(bakPath)
	if err := os.Rename(bakPath, rotated); err != nil {
		return "", fmt.Errorf("failed to rotate hosts backup %q: %w", bakPath, err)
	}
	commonLogger.Printf(
		"hosts: existing backup %q does not match the current hosts file; preserved as %q\n",
		bakPath,
		rotated,
	)
	return rotated, nil
}

// createHostsBackup writes the pre-setup state, without any of our lines.
func createHostsBackup(current []byte) (err error) {
	stripped, stripErr := StrippedContents(current)
	if stripErr != nil {
		return fmt.Errorf("failed to prepare hosts backup contents: %w", stripErr)
	}
	bakLock, openErr := OpenLockedBackup(os.O_RDWR | os.O_CREATE | os.O_TRUNC)
	if openErr != nil {
		return fmt.Errorf("failed to create hosts backup %q: %w", hostsBackupPath(), openErr)
	}
	name := bakLock.File.Name()
	defer func() {
		_ = bakLock.Unlock()
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if _, err = bakLock.File.Write(stripped); err != nil {
		return fmt.Errorf("failed to write hosts backup %q: %w", name, err)
	}
	if err = bakLock.File.Sync(); err != nil {
		return fmt.Errorf("failed to sync hosts backup %q: %w", name, err)
	}
	return nil
}

// ensureHostsBackup makes sure hosts.bak holds the state that preceded this
// setup run. A reusable backup is left untouched; anything else is rotated
// (never destroyed) and replaced with the current contents minus our lines.
func ensureHostsBackup(current []byte) error {
	bakPath := filepath.Join(filepath.Dir(pathFn()), "hosts.bak")
	bakLock, openErr := OpenLockedBackup(os.O_RDONLY)
	if openErr == nil {
		info, statErr := bakLock.File.Stat()
		if statErr != nil {
			_ = bakLock.Unlock()
			return fmt.Errorf("failed to inspect existing hosts backup %q: %w", bakPath, statErr)
		}
		if !info.Mode().IsRegular() {
			_ = bakLock.Unlock()
			return fmt.Errorf("existing hosts backup %q is not a regular file", bakPath)
		}
		backup, readErr := readHostsContents(bakLock.File)
		unlockErr := bakLock.Unlock()
		if readErr != nil {
			return fmt.Errorf("failed to read existing hosts backup %q: %w", bakPath, readErr)
		}
		if unlockErr != nil {
			return fmt.Errorf("failed to unlock existing hosts backup %q: %w", bakPath, unlockErr)
		}
		state, classifyErr := ClassifyBackup(current, backup)
		if classifyErr == nil {
			switch state {
			case BackupMatches, BackupPreEmpty, BackupCurrentLost:
				// A backup written by an older version may still contain our
				// own lines. Reusing it as-is would bring stale mappings back
				// on revert, so clean it in place.
				if contaminated, ownedErr := containsOwnedLines(backup); ownedErr == nil && contaminated {
					return createHostsBackup(backup)
				}
				return nil
			}
		}
		if _, rotateErr := rotateHostsBackup(bakPath); rotateErr != nil {
			return rotateErr
		}
	} else if !errors.Is(openErr, os.ErrNotExist) {
		return fmt.Errorf("failed to open existing hosts backup %q: %w", bakPath, openErr)
	}
	return createHostsBackup(current)
}

func AddHosts(ip net.IP, gameId string, hostFilePath string, lineEnding string, withMacOsExclusive bool, flushFn func() (result *exec.Result)) (ok bool, err error) {
	var systemHosts bool
	if hostFilePath == "" {
		systemHosts = true
		hostFilePath = pathFn()
	}
	if lineEnding == "" {
		lineEnding = LineEnding
	}
	var hostsFileLock *fileLock.Lock
	mappings := Mappings(gameId, ip, withMacOsExclusive)
	var restLines []Line
	hostsFileLock, err = openLockedHostsFile(hostFilePath, os.O_RDWR)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			hostsFileLock, err = openLockedHostsFile(hostFilePath, os.O_RDWR|os.O_CREATE)
		}
		if err != nil {
			err = fmt.Errorf("failed to open/lock hosts file %q (ip %q): %w", hostFilePath, ip.String(), err)
			return
		}
	}
	// Capture the pre-setup state before missingIpMappings consumes the file:
	// it is the source of the backup and the reference for its validation.
	current, readErr := readHostsContents(hostsFileLock.File)
	if readErr != nil {
		_ = hostsFileLock.Unlock()
		err = fmt.Errorf("failed to read hosts file %q (ip %q): %w", hostFilePath, ip.String(), readErr)
		return
	}
	//goland:noinspection ALL
	restLines, err = missingIpMappings(&mappings, hostsFileLock.File)
	if err != nil {
		_ = hostsFileLock.Unlock()
		err = fmt.Errorf("failed to parse hosts file %q (ip %q): %w", hostFilePath, ip.String(), err)
		return
	}
	if len(mappings) == 0 {
		_ = hostsFileLock.Unlock()
		ok = true
		return
	}
	_, err = hostsFileLock.File.Seek(0, io.SeekStart)
	if err != nil {
		_ = hostsFileLock.Unlock()
		err = fmt.Errorf("failed to seek hosts file %q: %w", hostFilePath, err)
		return
	}
	err = UpdateHosts(hostsFileLock, func(f *os.File) error {
		// Always rewrite the temp file from its start. UpdateHosts leaves the
		// offset at the end of the copied hosts, so appending there duplicated
		// the original contents whenever no backup branch re-seeked the file.
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return fmt.Errorf("failed to seek temp hosts file: %w", seekErr)
		}
		if truncateErr := f.Truncate(0); truncateErr != nil {
			return fmt.Errorf("failed to truncate temp hosts file: %w", truncateErr)
		}
		if systemHosts {
			if backupErr := ensureHostsBackup(current); backupErr != nil {
				return backupErr
			}
		}
		for _, line := range restLines {
			if _, err = f.WriteString(line.String()); err != nil {
				return fmt.Errorf("failed to write preserved hosts line: %w", err)
			}
			if _, err = f.WriteString(lineEnding); err != nil {
				return fmt.Errorf("failed to write hosts line ending: %w", err)
			}
		}
		_, err = f.WriteString(mappings.String(lineEnding))
		if err != nil {
			return fmt.Errorf("failed to write new hosts mappings (ip %q): %w", ip.String(), err)
		}
		return nil
	}, flushFn)
	if err != nil {
		err = fmt.Errorf("failed to update hosts file %q (ip %q): %w", hostFilePath, ip.String(), err)
	} else {
		ok = true
	}
	return
}
