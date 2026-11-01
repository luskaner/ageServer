//go:build linux || darwin

// This file provides shared Unix implementations for Linux and Darwin.
// It works in conjunction with:
//   - process_linux.go: provides GetProcessStartTime and ProcessesPID for Linux
//   - process_darwin.go: provides GetProcessStartTime and ProcessesPID for Darwin
//   - process_notwin.go: provides FindProcess and FindProcessWithStartTime for all non-Windows

package process

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func status(proc *os.Process) (err error, alive bool, permitted bool) {
	err = proc.Signal(unix.Signal(0))
	if err == nil {
		alive = true
		permitted = true
	} else if errors.Is(err, unix.EPERM) {
		alive = true
		err = nil
	}
	return
}

// WaitForProcess waits for proc to exit, and reports whether it did.
//
// A nil duration waits for as long as it takes.
func WaitForProcess(proc *os.Process, duration *time.Duration) bool {
	return WaitForProcessContext(context.Background(), proc, duration)
}

// WaitForProcessContext is WaitForProcess that gives up when ctx is cancelled.
//
// It exists because the longest wait in the launcher is for another instance to
// go away, which is exactly the wait somebody wants to interrupt: a launcher that
// is waiting up to a minute for a previous run cannot be stopped during it, and
// in a graphical frontend nothing can be done about that at all.
func WaitForProcessContext(ctx context.Context, proc *os.Process, duration *time.Duration) bool {
	pollInterval := 100 * time.Millisecond
	if duration == nil {
		pollInterval *= 10
	}
	waitByPolling := func(timeoutChan <-chan time.Time) bool {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			if _, alive, _ := status(proc); !alive {
				return true
			}
			select {
			case <-ctx.Done():
				return false
			case <-timeoutChan:
				return false
			case <-ticker.C:
				if _, alive, _ := status(proc); !alive {
					return true
				}
			}
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()
	var timeoutChan <-chan time.Time
	if duration != nil {
		timer := time.NewTimer(*duration)
		defer timer.Stop()
		timeoutChan = timer.C
	}
	select {
	case <-ctx.Done():
		return false
	case err := <-done:
		if err == nil {
			return true
		}
		return waitByPolling(timeoutChan)
	case <-timeoutChan:
		return false
	}
}
