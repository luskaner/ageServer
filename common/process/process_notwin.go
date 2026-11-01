//go:build !windows

// This file provides FindProcess and FindProcessWithStartTime for all non-Windows systems.
// It relies on GetProcessStartTime being defined in platform-specific files:
//   - process_linux.go for Linux
//   - process_darwin.go for Darwin
//   - process_other.go for other Unix systems (fallback)

package process

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func FindProcess(pid int) (proc *os.Process, err error) {
	return FindProcessWithStartTime(pid, 0)
}

func FindProcessWithStartTime(pid int, expectedStartTime int64) (proc *os.Process, err error) {
	proc, err = os.FindProcess(pid)
	if err != nil {
		return
	}
	var permitted bool
	if err, _, permitted = status(proc); err != nil {
		proc = nil
		return
	} else if !permitted {
		err = nil
	}
	if expectedStartTime != 0 {
		actualStartTime, startErr := GetProcessStartTime(pid)
		if startErr != nil {
			if errors.Is(startErr, unix.EPERM) {
				// Process exists but we can't validate start time due to permissions
				// Treat as valid since Signal(0) already confirmed existence
				return
			}
			proc = nil
			err = startErr
			return
		}
		if actualStartTime != expectedStartTime {
			proc = nil
			err = errors.New("process start time mismatch")
			return
		}
	}
	return
}

// On this platform os.Process.Signal(os.Interrupt) works, so KillProc may try to
// be polite before resorting to Kill.
const platformSupportsGracefulSignal = true
