package exec

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// Split out of executor_test.go: these reach for ShellExecuteExW seams, fixArgs
// and windows.Handle, none of which exist off Windows. Leaving them in the
// shared file made the whole package's test build fail everywhere else, and four
// other packages inherit that failure through their test imports.
//
// Bodies are verbatim from executor_test.go; only the location changed.
func TestExecAdminPath(t *testing.T) {
	origCall := shellExecuteExCallFn
	origPid := getProcessIdFn
	SetShellExecuteExCallFn(func(_ *SHELLEXECUTEINFO) (uintptr, uintptr, error) { return 1, 0, nil })
	SetGetProcessIdFn(func(h windows.Handle) (uint32, error) { return 1234, nil })
	defer func() { SetShellExecuteExCallFn(origCall); SetGetProcessIdFn(origPid) }()
	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		AsAdmin:    true,
		ShowWindow: true,
		Wait:       false,
		Pid:        true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec admin path should succeed with mock, got %v", r.Err)
	}
	if r.Pid != 1234 {
		t.Errorf("Pid = %d, want 1234", r.Pid)
	}
}

func TestShellExecuteExFailure(t *testing.T) {
	// Mock ShellExecuteEx to return 0 (failure) with error
	original := shellExecuteExCallFn
	SetShellExecuteExCallFn(func(_ *SHELLEXECUTEINFO) (uintptr, uintptr, error) {
		return 0, 0, errors.New("shell execute failed")
	})
	defer SetShellExecuteExCallFn(original)

	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       false,
		Pid:        true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err == nil {
		t.Error("expected error when ShellExecuteEx fails")
	}
}

func TestWaitForSingleObjectError(t *testing.T) {
	// Mock WaitForSingleObject to return error
	original := waitForSingleObjectFn
	SetWaitForSingleObjectFn(func(h windows.Handle, dwMilliseconds uint32) (uint32, error) {
		return 0, errors.New("wait failed")
	})
	defer SetWaitForSingleObjectFn(original)

	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       true,
		ExitCode:   true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err == nil {
		t.Error("expected error when WaitForSingleObject fails")
	}
}

func TestGetExitCodeProcessError(t *testing.T) {
	// Mock GetExitCodeProcess to return error
	original := getExitCodeProcessFn
	SetGetExitCodeProcessFn(func(h windows.Handle, exitCode *uint32) error {
		return errors.New("get exit code failed")
	})
	defer SetGetExitCodeProcessFn(original)

	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       true,
		ExitCode:   true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err == nil {
		t.Error("expected error when GetExitCodeProcess fails")
	}
}

func TestGetProcessIdError(t *testing.T) {
	// Mock GetProcessId to return error
	original := getProcessIdFn
	SetGetProcessIdFn(func(h windows.Handle) (uint32, error) {
		return 0, errors.New("get pid failed")
	})
	defer SetGetProcessIdFn(original)

	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       false,
		Pid:        true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err == nil {
		t.Error("expected error when GetProcessId fails")
	}
}

func TestSetShellExecuteExCallFnNil(t *testing.T) {
	restore := SetShellExecuteExCallFn(nil)
	defer restore()
	if shellExecuteExCallFn == nil {
		t.Error("shellExecuteExCallFn should not be nil after Set(nil)")
	}
}

func TestFixArgs(t *testing.T) {
	result := fixArgs("hello", "world")
	if len(result) != 2 {
		t.Fatalf("fixArgs returned %d args, want 2", len(result))
	}
	if result[0] != `"hello"` {
		t.Errorf("fixArgs[0] = %q, want %q", result[0], `"hello"`)
	}
	if result[1] != `"world"` {
		t.Errorf("fixArgs[1] = %q, want %q", result[1], `"world"`)
	}
}

func TestFixArgsWithQuotes(t *testing.T) {
	result := fixArgs(`say "hi"`)
	expected := `"say \"hi\""`
	if result[0] != expected {
		t.Errorf("fixArgs with quotes = %q, want %q", result[0], expected)
	}
}

func TestFixArgsEmpty(t *testing.T) {
	result := fixArgs()
	if len(result) != 0 {
		t.Errorf("fixArgs() with no args returned %d, want 0", len(result))
	}
}
