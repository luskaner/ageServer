package fileLock

import (
	"encoding/binary"
	"errors"
	"os"

	"github.com/luskaner/ageServer/common/process"
)

// Locker is the seam used by launcher to make PidLock testable without touching filesystem.
type Locker interface {
	Lock() error
	Unlock() error
}

var ErrAlreadyRunning = errors.New("another instance is already running")

var (
	openFileFn = openFile
	writePidFn = writePid
	// Injectables for tests.
	osExecutableFn   = os.Executable
	processProcessFn = process.Process
	fileTruncateFn   = func(f *os.File, size int64) error { return f.Truncate(size) }
	fileWriteFn      = func(f *os.File, data []byte) (int, error) { return f.Write(data) }
)

func openFile() (err error, f *os.File) {
	var exe string
	exe, err = osExecutableFn()
	if err != nil {
		return
	}
	var pidPath string
	var proc *os.Process
	pidPath, proc, err = processProcessFn(exe)
	if err != nil {
		return
	}
	if proc != nil {
		err = ErrAlreadyRunning
		return
	}
	f, err = os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY, 0644)
	return
}

func writePid(f *os.File) error {
	pid := os.Getpid()
	// If GetProcessStartTime fails, use 0 which disables start time validation
	// but still allows the lock to function based on PID alone
	startTime, _ := process.GetProcessStartTime(pid)

	data := make([]byte, process.PidFileSize)
	binary.LittleEndian.PutUint64(data[0:8], uint64(pid))
	binary.LittleEndian.PutUint64(data[8:16], uint64(startTime))

	err := fileTruncateFn(f, int64(len(data)))
	if err != nil {
		return err
	}
	_, err = fileWriteFn(f, data)
	if err != nil {
		return err
	}
	return f.Sync()
}

func removeFile(name string) error {
	err := os.Remove(name)
	if err != nil {
		return err
	}
	return nil
}

type PidLock struct {
	fileLock Lock
}

func (l *PidLock) Lock() error {
	//goland:noinspection ALL
	err, file := openFileFn()
	if err != nil {
		return err
	}
	err = l.fileLock.Lock(file)
	if err != nil {
		return err
	}
	err = writePidFn(file)
	if err != nil {
		l.fileLock.clean()
		return err
	}
	return nil
}

func (l *PidLock) Unlock() error {
	if l.fileLock.BaseLock == nil || l.fileLock.BaseLock.File == nil {
		return nil
	}
	name := l.fileLock.BaseLock.File.Name()
	defer l.fileLock.clean()
	if err := l.fileLock.Unlock(); err != nil {
		return err
	}
	return removeFile(name)
}
