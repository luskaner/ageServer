package custom

import (
	"testing"

	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
)

func TestString(t *testing.T) {
	e := Exec{Executable: "/some/path/app.exe"}
	if got := e.String(); got != "Custom Path" {
		t.Errorf("String() = %q, want %q", got, "Custom Path")
	}
}

func TestGameProcesses(t *testing.T) {
	e := Exec{Executable: "test"}
	steam, _, xbox := e.GameProcesses()
	if !steam {
		t.Error("steamProcess should be true")
	}
	// xbox depends on platform, just verify it doesn't panic
	_ = xbox
}

func TestDo(t *testing.T) {
	orig := execFn
	defer func() { execFn = orig }()
	var captured commonExecutor.Options
	execFn = func(o commonExecutor.Options) *commonExecutor.Result {
		captured = o
		return &commonExecutor.Result{Pid: 1234}
	}
	e := Exec{Executable: "C:\\test.exe"}
	var called bool
	result := e.Do([]string{"arg1"}, func(opts commonExecutor.Options) {
		called = true
		if opts.File != "C:\\test.exe" {
			t.Errorf("File = %q, want %q", opts.File, "C:\\test.exe")
		}
	})
	if !called {
		t.Error("optionsFn should be called")
	}
	if captured.File != "C:\\test.exe" {
		t.Errorf("execFn File = %q, want %q", captured.File, "C:\\test.exe")
	}
	if len(captured.Args) != 1 || captured.Args[0] != "arg1" {
		t.Errorf("Args = %v, want [arg1]", captured.Args)
	}
	if !captured.Pid {
		t.Error("Pid should be true")
	}
	if captured.AsAdmin {
		t.Error("AsAdmin should be false for Do")
	}
	if !captured.ShowWindow || !captured.GUI {
		t.Error("ShowWindow and GUI should be true")
	}
	if result.Pid != 1234 {
		t.Errorf("Pid = %d, want 1234", result.Pid)
	}
}

func TestDoElevated(t *testing.T) {
	orig := execFn
	defer func() { execFn = orig }()
	var captured commonExecutor.Options
	execFn = func(o commonExecutor.Options) *commonExecutor.Result {
		captured = o
		return &commonExecutor.Result{Pid: 5678}
	}
	e := Exec{Executable: "C:\\test.exe"}
	result := e.DoElevated([]string{"arg1"}, func(opts commonExecutor.Options) {})
	if !captured.AsAdmin {
		t.Error("AsAdmin should be true for DoElevated")
	}
	if result.Pid != 5678 {
		t.Errorf("Pid = %d, want 5678", result.Pid)
	}
}

func TestDoDefaultExecFnEmptyFile(t *testing.T) {
	// Keeps the real default execFn (the package-level closure); empty File makes
	// Exec() return a validation error without launching any process (CI-safe).
	e := Exec{Executable: ""}
	r := e.Do(nil, func(commonExecutor.Options) {})
	if r.Err == nil || r.Err.Error() != "no file specified" {
		t.Fatalf("expected no-file error, got %v", r.Err)
	}
}
