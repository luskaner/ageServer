package exec

import (
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/luskaner/ageServer/common"
	"golang.org/x/sys/windows"
)

var (
	modshell32         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = modshell32.NewProc("ShellExecuteExW")
)

// Injectable Windows API functions for testing
var (
	waitForSingleObjectFn = windows.WaitForSingleObject
	getExitCodeProcessFn  = windows.GetExitCodeProcess
	getProcessIdFn        = windows.GetProcessId
	closeHandleFn         = windows.CloseHandle
)

// shellExecuteExCall is the seam for ShellExecuteExW. It takes the struct by
// pointer rather than as a variadic uintptr so a test double can reach hProcess
// without converting a uintptr back into an unsafe.Pointer, which vet rejects
// and which is only sound while the pointed-to object is pinned.
type shellExecuteExCall func(info *SHELLEXECUTEINFO) (uintptr, uintptr, error)

// callShellExecuteEx is the real ShellExecuteExW. The caller keeps info pinned
// for the duration, so the conversion here cannot outlive the object.
func callShellExecuteEx(info *SHELLEXECUTEINFO) (uintptr, uintptr, error) {
	return procShellExecuteEx.Call(uintptr(unsafe.Pointer(info)), 0, 0)
}

var shellExecuteExCallFn shellExecuteExCall = callShellExecuteEx

// SetWaitForSingleObjectFn sets a custom WaitForSingleObject for testing.
func SetWaitForSingleObjectFn(fn func(h windows.Handle, dwMilliseconds uint32) (uint32, error)) {
	waitForSingleObjectFn = fn
}

// SetGetExitCodeProcessFn sets a custom GetExitCodeProcess for testing.
func SetGetExitCodeProcessFn(fn func(h windows.Handle, exitCode *uint32) error) {
	getExitCodeProcessFn = fn
}

// SetGetProcessIdFn sets a custom GetProcessId for testing.
func SetGetProcessIdFn(fn func(h windows.Handle) (uint32, error)) {
	getProcessIdFn = fn
}

// SetShellExecuteExCallFn sets a custom ShellExecuteEx call for testing.
func SetShellExecuteExCallFn(fn func(info *SHELLEXECUTEINFO) (uintptr, uintptr, error)) (restore func()) {
	orig := shellExecuteExCallFn
	if fn == nil {
		shellExecuteExCallFn = callShellExecuteEx
	} else {
		shellExecuteExCallFn = fn
	}
	return func() { shellExecuteExCallFn = orig }
}

// SetCloseHandleFn sets a custom CloseHandle for testing.
func SetCloseHandleFn(fn func(windows.Handle) error) (restore func()) {
	orig := closeHandleFn
	if fn == nil {
		closeHandleFn = windows.CloseHandle
	} else {
		closeHandleFn = fn
	}
	return func() { closeHandleFn = orig }
}

type SHELLEXECUTEINFO struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

func shellExecuteEx(verb string, start bool, executable string, executableWorkingPath bool, getPid bool, show int32, arg ...string) (err error, pid uint32, exitCode int) {
	pid = 0
	exitCode = common.ErrSuccess
	verbPtr, _ := windows.UTF16PtrFromString(verb)
	exe, _ := windows.UTF16PtrFromString(executable)
	args, _ := windows.UTF16PtrFromString(strings.Join(fixArgs(arg...), " "))

	// SHELLEXECUTEINFO is an in/out parameter: ShellExecuteExW writes hProcess,
	// hInstApp and friends back into it. It used to be a stack allocation whose
	// address was handed to the API as a uintptr, so if the goroutine's stack
	// grew while the call was in flight the OS wrote into the old, discarded
	// stack and this copy kept hProcess == 0. The symptom was
	// GetProcessId(0) failing with ERROR_INVALID_HANDLE, or a bogus exit code
	// on the Wait path, which is the runas path used to start the elevated
	// 'config-admin-agent'. Pinning forces the struct onto the heap (where the
	// collector does not move it) and keeps it, and the UTF-16 buffers its
	// fields reference, alive for the duration of the call.
	var info SHELLEXECUTEINFO
	var pinner runtime.Pinner
	pinner.Pin(&info)
	info.cbSize = uint32(unsafe.Sizeof(SHELLEXECUTEINFO{}))
	info.fMask = 0x00000040 // SEE_MASK_NOCLOSEPROCESS
	info.lpVerb = verbPtr
	info.lpFile = exe
	info.lpParameters = args
	info.nShow = show

	if executableWorkingPath {
		info.lpDirectory, _ = windows.UTF16PtrFromString(filepath.Dir(executable))
	}

	ret, _, err := shellExecuteExCallFn(&info)
	// Copy out what the API wrote before letting the pin go: once Unpin runs the
	// struct is free to be collected, and hProcess is still read below.
	hProcess := info.hProcess
	pinner.Unpin()
	runtime.KeepAlive(&info)
	// SEE_MASK_NOCLOSEPROCESS makes the API hand us ownership of a process
	// handle, and nothing in this function hands it back: the Wait path uses it
	// for the exit code and the Pid path only needs the number. It was leaked on
	// every spawn, so a session that elevates config-admin-agent a few times
	// accumulated handles for the lifetime of the process.
	defer func(h windows.Handle) {
		if h != 0 {
			_ = closeHandleFn(h)
		}
	}(hProcess)
	if ret == 0 {
		return
	}

	err = nil

	if !start {
		_, err = waitForSingleObjectFn(hProcess, windows.INFINITE)
		if err != nil {
			return
		}
		var tmpExitCode uint32
		err = getExitCodeProcessFn(hProcess, &tmpExitCode)
		if err != nil {
			return
		}
		exitCode = int(tmpExitCode)
	} else if getPid {
		pid, err = getProcessIdFn(hProcess)
	}

	return
}
