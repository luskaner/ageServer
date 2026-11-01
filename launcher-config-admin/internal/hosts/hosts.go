package hosts

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/luskaner/ageServer/common/fileLock"
	commonHosts "github.com/luskaner/ageServer/common/hosts"
	"github.com/luskaner/ageServer/common/hosts/text"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

func restoreBackup(backFile *os.File, mainLock *fileLock.Lock) error {
	return commonHosts.UpdateHosts(mainLock, func(f *os.File) error {
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		var written int64
		written, err = io.Copy(f, backFile)
		if err != nil {
			return err
		}
		return f.Truncate(written)
	}, FlushDns)
}

func restoreInPlace(mainLock *fileLock.Lock) error {
	buf, err := io.ReadAll(mainLock.File)
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
	return commonHosts.UpdateHosts(mainLock, func(f *os.File) error {
		var lines []string
		var line string

		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		decodingReader := transform.NewReader(f, enc.NewDecoder())
		scanner := bufio.NewScanner(decodingReader)
		for scanner.Scan() {
			line = scanner.Text()
			ok, _, parsedLine := commonHosts.ParseLine(line, true)
			if !ok {
				continue
			}
			var ignoreLine bool
			if parsedLine.Own() {
				if parsedLine.OnlyComments() {
					line = parsedLine.WithoutOwnMarking().Uncommented()
				} else {
					ignoreLine = true
				}
			}
			if !ignoreLine {
				lines = append(lines, line)
			}
		}

		if err = scanner.Err(); err != nil {
			return err
		}

		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		linesJoined := strings.Join(lines, commonHosts.LineEnding)
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
		} else if bakInfo, err := bakLock.File.Stat(); err != nil {
			_ = mainLock.Unlock()
			return err
		} else if mainInfo, err := mainLock.File.Stat(); err != nil {
			_ = mainLock.Unlock()
			return err
		} else {
			doRestoreBak = mainInfo.ModTime().Before(bakInfo.ModTime().Add(1 * time.Second))
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
