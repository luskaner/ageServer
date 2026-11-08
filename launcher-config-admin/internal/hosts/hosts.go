package hosts

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/luskaner/ageServer/common/fileLock"
	commonHosts "github.com/luskaner/ageServer/common/hosts"
	"github.com/luskaner/ageServer/common/hosts/text"
	"golang.org/x/text/encoding"
)

// readAllContents reads the whole file and leaves the offset at its start.
func readAllContents(f *os.File) ([]byte, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return buf, nil
}

func restoreBackup(backFile *os.File, mainLock *fileLock.Lock) error {
	return commonHosts.UpdateHosts(mainLock, func(f *os.File) error {
		if _, err := backFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		var written int64
		written, err := io.Copy(f, backFile)
		if err != nil {
			return err
		}
		return f.Truncate(written)
	}, FlushDns)
}

// ownedActiveHosts returns every host that is actively mapped by an AgeServer
// line in the file.
func ownedActiveHosts(lines []string) map[commonHosts.Host]struct{} {
	owned := make(map[commonHosts.Host]struct{})
	for _, line := range lines {
		ok, _, parsedLine := commonHosts.ParseLine(line, true)
		if !ok || !parsedLine.Own() || parsedLine.OnlyComments() {
			continue
		}
		for _, host := range parsedLine.Hosts() {
			owned[host] = struct{}{}
		}
	}
	return owned
}

func hostsCovered(hosts []commonHosts.Host, active map[commonHosts.Host]struct{}) bool {
	if len(hosts) == 0 {
		return false
	}
	for _, host := range hosts {
		if _, ok := active[host]; !ok {
			return false
		}
	}
	return true
}

func restoreInPlace(mainLock *fileLock.Lock) error {
	buf, err := readAllContents(mainLock.File)
	if err != nil {
		_ = mainLock.Unlock()
		return err
	}
	encType := text.Encoding(buf)
	var enc encoding.Encoding
	enc, err = text.GetEncoding(encType)
	if err != nil {
		_ = mainLock.Unlock()
		return err
	}
	_, err = mainLock.File.Seek(0, io.SeekStart)
	if err != nil {
		_ = mainLock.Unlock()
		return err
	}
	lines := make([]string, 0)
	decoded, err := enc.NewDecoder().String(string(buf))
	if err != nil {
		_ = mainLock.Unlock()
		return err
	}
	scanner := bufio.NewScanner(strings.NewReader(decoded))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		_ = mainLock.Unlock()
		return err
	}
	activeOwned := ownedActiveHosts(lines)

	return commonHosts.UpdateHosts(mainLock, func(f *os.File) error {
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		kept := make([]string, 0, len(lines))
		for _, line := range lines {
			ok, _, parsedLine := commonHosts.ParseLine(line, true)
			if !ok {
				continue
			}
			var ignoreLine bool
			if parsedLine.Own() {
				if parsedLine.OnlyComments() {
					inner := parsedLine.WithoutOwnMarking().Uncommented()
					if parsedLine.Preserved() {
						// We commented this copy ourselves: a revert restores it.
						if innerOk, _, reparsed := commonHosts.ParseLine(inner, true); innerOk && !reparsed.OnlyComments() {
							line = inner
						} else {
							ignoreLine = true
						}
					} else if innerOk, _, reparsed := commonHosts.ParseLine(inner, true); innerOk &&
						!reparsed.OnlyComments() &&
						hostsCovered(reparsed.Hosts(), activeOwned) {
						// Legacy comment: only revive it when the active owned
						// lines already cover exactly the same hosts (it is a
						// copy of one of them). An orphaned legacy comment is
						// dropped instead of becoming an active mapping.
						line = inner
					} else {
						ignoreLine = true
					}
				} else {
					ignoreLine = true
				}
			}
			if !ignoreLine {
				kept = append(kept, line)
			}
		}

		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		linesJoined := strings.Join(kept, commonHosts.LineEnding)
		var linesJoinedEncoded string
		linesJoinedEncoded, err = enc.NewEncoder().String(linesJoined)
		if err != nil {
			return err
		}
		var n int
		n, err = f.WriteString(linesJoinedEncoded)
		if err != nil {
			return err
		}

		err = f.Truncate(int64(n))
		if err != nil {
			return err
		}
		return nil
	}, FlushDns)
}

func RemoveHosts() error {
	var bakExists bool
	var removeBak bool
	bakLock, err := commonHosts.OpenLockedBackup(os.O_RDWR)
	if err == nil {
		bakExists = true
		defer func() {
			backLockName := bakLock.File.Name()
			_ = bakLock.Unlock()
			if removeBak {
				_ = os.Remove(backLockName)
			}
		}()
	}
	mainLock, err := commonHosts.OpenLockedMain(os.O_RDWR)
	var createdMain bool
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if bakExists {
				mainLock, err = commonHosts.OpenLockedMain(os.O_RDWR | os.O_CREATE)
				if err != nil {
					return err
				}
				createdMain = true
			} else {
				return nil
			}
		} else {
			return err
		}
	}
	if bakExists {
		var doRestoreBak bool
		if createdMain {
			doRestoreBak = true
		} else {
			currentBytes, readErr := readAllContents(mainLock.File)
			if readErr != nil {
				_ = mainLock.Unlock()
				return readErr
			}
			backupBytes, readErr := readAllContents(bakLock.File)
			if readErr != nil {
				_ = mainLock.Unlock()
				return readErr
			}
			// Only a validated backup may replace the whole hosts file. When the
			// backup is foreign the file is cleaned in place instead, so an
			// unrelated hosts.bak can never truncate the user's mappings.
			if state, classifyErr := commonHosts.ClassifyBackup(currentBytes, backupBytes); classifyErr == nil {
				switch state {
				case commonHosts.BackupMatches, commonHosts.BackupPreEmpty, commonHosts.BackupCurrentLost:
					doRestoreBak = true
				}
			}
		}
		if doRestoreBak {
			err = restoreBackup(bakLock.File, mainLock)
			if err == nil {
				removeBak = true
			}
			return err
		}
		removeBak = true
	}
	return restoreInPlace(mainLock)
}
