package process

import (
	"context"
	"errors"
	"os"
	"slices"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WaitForProcess waits for proc to exit, and reports whether it did.
//
// A nil duration waits for as long as it takes.
func WaitForProcess(proc *os.Process, duration *time.Duration) bool {
	return WaitForProcessContext(context.Background(), proc, duration)
}

// WaitForProcessContext is WaitForProcess that gives up when ctx is cancelled.
//
// On Windows there is no way to wait on a process and on a cancellation at the
// same time without building an event for it, so the wait is taken in slices and
// the context is looked at between them. The slices are short enough that a stop
// is noticed at once and long enough that a process that exits is not noticed
// late.
func WaitForProcessContext(ctx context.Context, proc *os.Process, duration *time.Duration) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, true, uint32(proc.Pid))
	if err != nil {
		return false
	}

	defer func(handle windows.Handle) {
		_ = windows.CloseHandle(handle)
	}(handle)

	const slice = 100 * time.Millisecond
	var deadline time.Time
	if duration != nil {
		deadline = time.Now().Add(*duration)
	}
	for {
		if err := ctx.Err(); err != nil {
			return false
		}
		wait := slice
		if deadline.IsZero() {
			// Wait out the whole thing in one go only when there is nothing to
			// wake up for. A context with no deadline can still be cancelled, and
			// a cancellation is exactly what this wait is blocking on: the
			// INFINITE call below returns when the process exits, and would sit
			// there for the rest of the session if the caller gave up waiting.
			//
			// Only context.Background has a nil Done channel, so that, and only
			// that, is the case where waiting in slices is pure overhead.
			if _, ok := ctx.Deadline(); !ok && ctx.Done() == nil {
				event, err := windows.WaitForSingleObject(handle, windows.INFINITE)
				return err == nil && event == uint32(windows.WAIT_OBJECT_0)
			}
		} else if remaining := time.Until(deadline); remaining <= 0 {
			return false
		} else if remaining < wait {
			wait = remaining
		}
		event, err := windows.WaitForSingleObject(handle, uint32(wait.Milliseconds()))
		if err != nil {
			return false
		}
		if event == uint32(windows.WAIT_OBJECT_0) {
			return true
		}
	}
}

// ProcessesByNames returns a map of process names to their procs.
// Note: If multiple processes share the same name, only one PID is stored per name.
func ProcessesByNames(names []string) map[string]*os.Process {
	name := func(entry *windows.ProcessEntry32) string {
		return windows.UTF16ToString(entry.ExeFile[:])
	}
	entries := processesEntry(func(entry *windows.ProcessEntry32) bool {
		return slices.Contains(names, name(entry))
	}, false)
	processes := make(map[string]*os.Process)
	for _, entry := range entries {
		processes[name(&entry)] = &os.Process{Pid: int(entry.ProcessID)}
	}
	return processes
}

func processesEntry(matches func(entry *windows.ProcessEntry32) bool, firstOnly bool) []windows.ProcessEntry32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer func(handle windows.Handle) {
		_ = windows.CloseHandle(handle)
	}(snapshot)

	var procEntry windows.ProcessEntry32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))

	err = windows.Process32First(snapshot, &procEntry)
	if err != nil {
		return nil
	}

	var entries []windows.ProcessEntry32

	for {
		if matches(&procEntry) {
			entries = append(entries, procEntry)
			if firstOnly {
				break
			}
		}
		err = windows.Process32Next(snapshot, &procEntry)
		if err != nil {
			break
		}
	}

	return entries
}

func GetProcessStartTime(pid int) (int64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer func(handle windows.Handle) {
		_ = windows.CloseHandle(handle)
	}(handle)

	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	err = windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return 0, err
	}
	return creationTime.Nanoseconds(), nil
}

func FindProcess(pid int) (proc *os.Process, err error) {
	return FindProcessWithStartTime(pid, 0)
}

func FindProcessWithStartTime(pid int, expectedStartTime int64) (proc *os.Process, err error) {
	proc, err = os.FindProcess(pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		err = nil
	}
	entries := processesEntry(func(entry *windows.ProcessEntry32) bool {
		return int(entry.ProcessID) == pid
	}, true)
	if len(entries) == 0 {
		proc = nil
		return
	}
	if err != nil {
		proc = &os.Process{Pid: pid}
		err = nil
	}
	if expectedStartTime != 0 {
		actualStartTime, startErr := GetProcessStartTime(pid)
		if startErr != nil {
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

// Windows has no graceful signal: os.Process.Signal only accepts os.Kill, so
// KillProc goes straight to TerminateProcess.
const platformSupportsGracefulSignal = false
