package hosts

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/fileLock"
	"github.com/luskaner/ageServer/common/hosts/text"
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

func missingIpMappings(mappings *HostMappings, hostFile *os.File) (restLines []Line, err error) {
	var scanner *bufio.Scanner
	err, _, scanner = decodeAndGetScanner(hostFile)
	if err != nil {
		return
	}
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		ok, overLimit, lineParsed := ParseLine(line, false)
		if !ok {
			ok, lineParsed = commentLineStr(line)
			if !ok {
				continue
			}
		}
		if lineParsed.OnlyComments() {
			restLines = append(restLines, lineParsed)
			continue
		}
		lineIp := lineParsed.IP()
		hosts := lineParsed.Hosts()
		indexesToAvoid := mapset.NewThreadUnsafeSet[int]()
		for i, lineHost := range hosts {
			if ip, ok := mappings.Get(lineHost); ok {
				if ip.Equal(lineIp) {
					mappings.Delete(lineHost)
				} else {
					indexesToAvoid.Add(i)
				}
			}
		}
		var keptHosts []Host
		var removedHosts bool
		for i := range hosts {
			if indexesToAvoid.ContainsOne(i) || commentHost(hosts[i]) {
				removedHosts = true
			} else {
				keptHosts = append(keptHosts, hosts[i])
			}
		}
		if overLimit {
			ok, lineParsed = commentLineStr(line)
			if ok {
				removedHosts = true
			}
		}
		if removedHosts {
			var nl Line
			if ok, nl = lineParsed.Commented(); ok {
				restLines = append(restLines, nl.WithOwnMarking())
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
		if systemHosts {
			var bakLock *fileLock.Lock
			bakLock, err = OpenLockedBackup(os.O_RDWR | os.O_CREATE | os.O_EXCL)
			if err != nil {
				if errors.Is(err, os.ErrExist) {
					return fmt.Errorf("failed to create hosts backup %q: file already exists (delete it after verifying its contents if the previous run failed): %w", filepath.Join(filepath.Dir(Path()), "hosts.bak"), err)
				}
				return fmt.Errorf("failed to create hosts backup %q: %w", filepath.Join(filepath.Dir(Path()), "hosts.bak"), err)
			}
			clearBakFunc := func() {
				filePath := bakLock.File.Name()
				_ = bakLock.Unlock()
				_ = os.Remove(filePath)
			}
			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				clearBakFunc()
				return fmt.Errorf("failed to seek temp hosts file before backup: %w", err)
			}
			if _, err = io.Copy(bakLock.File, f); err != nil {
				clearBakFunc()
				return fmt.Errorf("failed to copy hosts contents to backup: %w", err)
			}
			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				clearBakFunc()
				return fmt.Errorf("failed to seek temp hosts file after backup: %w", err)
			}
			_ = bakLock.Unlock()
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
