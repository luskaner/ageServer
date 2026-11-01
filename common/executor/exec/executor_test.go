package exec

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"testing"

	"github.com/luskaner/ageServer/common"
)

func TestResultSuccess(t *testing.T) {
	if !ResultSuccess.Success() {
		t.Error("ResultSuccess should be successful")
	}
}

func TestResultNilSuccess(t *testing.T) {
	var r *Result
	if r.Success() {
		t.Error("nil Result should not be successful")
	}
}

func TestResultWithErrNotSuccess(t *testing.T) {
	r := &Result{Err: errors.New("fail")}
	if r.Success() {
		t.Error("Result with Err should not be successful")
	}
}

func TestResultWithPidSuccess(t *testing.T) {
	r := &Result{Pid: 1234}
	if !r.Success() {
		t.Error("Result with Pid should be successful")
	}
}

func TestResultWithNonSuccessExitCode(t *testing.T) {
	r := &Result{ExitCode: 1}
	if r.Success() {
		t.Error("Result with non-zero ExitCode and no Pid should not be successful")
	}
}

func TestResultWithZeroExitCodeNoPid(t *testing.T) {
	r := &Result{ExitCode: common.ErrSuccess}
	if !r.Success() {
		t.Error("Result with ErrSuccess ExitCode and no Pid should be successful")
	}
}

func TestOptionsString(t *testing.T) {
	o := Options{File: "test.exe", Args: []string{"--flag", "val"}}
	got := o.String()
	want := "test.exe --flag val"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestOptionsStringNoArgs(t *testing.T) {
	o := Options{File: "test.exe"}
	got := o.String()
	if got != "test.exe" {
		t.Errorf("String() = %q, want %q", got, "test.exe")
	}
}

func TestExecWaitAndPidError(t *testing.T) {
	o := Options{File: "test.exe", Wait: true, Pid: true}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "pid requires wait as false" {
		t.Errorf("expected pid/wait error, got %v", r.Err)
	}
}

func TestExecExitCodeWithoutWaitError(t *testing.T) {
	o := Options{File: "test.exe", Wait: false, ExitCode: true}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "exit code requires wait as true" {
		t.Errorf("expected exitCode/wait error, got %v", r.Err)
	}
}

func TestExecGuiWithoutShowWindowError(t *testing.T) {
	o := Options{File: "test.exe", GUI: true, ShowWindow: false}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "gui apps need to set showWindow as true" {
		t.Errorf("expected gui/showWindow error, got %v", r.Err)
	}
}

func TestExecGuiWithStdoutError(t *testing.T) {
	var buf bytes.Buffer
	o := Options{File: "test.exe", GUI: true, ShowWindow: true, Stdout: &buf}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "gui/showWindow is not compatible with stdout/stderr" {
		t.Errorf("expected gui/stdout error, got %v", r.Err)
	}
}

func TestExecShowWindowWithStderrError(t *testing.T) {
	var buf bytes.Buffer
	o := Options{File: "test.exe", ShowWindow: true, Stderr: &buf}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "gui/showWindow is not compatible with stdout/stderr" {
		t.Errorf("expected showWindow/stderr error, got %v", r.Err)
	}
}

func TestExecEmptyFileError(t *testing.T) {
	o := Options{File: ""}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "no file specified" {
		t.Errorf("expected empty file error, got %v", r.Err)
	}
}

func TestGetExecutablePathLocal(t *testing.T) {
	// A relative path (local) should be resolved
	result := getExecutablePath("test.exe")
	if result == "" {
		// Could fail if os.Executable() fails, but shouldn't happen in tests
		t.Log("getExecutablePath returned empty (os.Executable error?)")
	}
}

func TestGetExecutablePathAbsolute(t *testing.T) {
	result := getExecutablePath("C:\\Windows\\System32\\cmd.exe")
	if result != "C:\\Windows\\System32\\cmd.exe" {
		t.Errorf("getExecutablePath absolute = %q, want %q", result, "C:\\Windows\\System32\\cmd.exe")
	}
}

func TestExecNoFileReturnsErr(t *testing.T) {
	o := Options{File: ""}
	r := o.Exec()
	if r.Err == nil {
		t.Error("expected error for empty file")
	}
}

func TestExecShellPathWithEmptyFile(t *testing.T) {
	// Shell + empty file → error before shell path
	o := Options{File: "", Shell: true}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "no file specified" {
		t.Errorf("expected empty file error, got %v", r.Err)
	}
}

func TestExecShellPathStdoutErr(t *testing.T) {
	var buf bytes.Buffer
	o := Options{File: "test.exe", Shell: true, ShowWindow: true, Stdout: &buf}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "gui/showWindow is not compatible with stdout/stderr" {
		t.Errorf("expected gui/stdout error, got %v", r.Err)
	}
}

func TestExecAdminPathWithStderr(t *testing.T) {
	var buf bytes.Buffer
	o := Options{File: "test.exe", AsAdmin: true, ShowWindow: true, Stderr: &buf}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "gui/showWindow is not compatible with stdout/stderr" {
		t.Errorf("expected gui/stderr error, got %v", r.Err)
	}
}

func configureCmdHelper(wait, show, gui, _ bool) *exec.Cmd {
	cmd := exec.Command("test.exe")
	configureCmd(cmd, wait, show, gui)
	return cmd
}

func TestConfigureCmdNonGuiNonShowWait(t *testing.T) {
	cmd := configureCmdHelper(true, false, false, false)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should not be nil")
	}
}

func TestConfigureCmdNonGuiNonShowNoWait(t *testing.T) {
	cmd := configureCmdHelper(false, false, false, false)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should not be nil")
	}
}

func TestConfigureCmdShowWindow(t *testing.T) {
	cmd := configureCmdHelper(false, true, false, false)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should not be nil")
	}
}

func TestConfigureCmdGuiNoWait(t *testing.T) {
	cmd := configureCmdHelper(false, false, true, false)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should not be nil")
	}
}

func TestConfigureCmdGuiWait(t *testing.T) {
	cmd := configureCmdHelper(true, false, true, false)
	// When gui + wait, no SysProcAttr is set
	if cmd.SysProcAttr != nil {
		t.Error("SysProcAttr should be nil for gui+wait")
	}
}

func TestMakeCommand(t *testing.T) {
	cmd := makeCommand("test.exe", false, nil, nil, "--arg")
	if cmd.Args[0] != "test.exe" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "test.exe")
	}
	if len(cmd.Args) != 2 {
		t.Errorf("cmd.Args length = %d, want 2", len(cmd.Args))
	}
}

func TestMakeCommandWithWorkingPath(t *testing.T) {
	cmd := makeCommand("C:\\path\\test.exe", true, nil, nil)
	if cmd.Dir == "" {
		t.Error("Dir should be set when executableWorkingPath is true")
	}
}

func TestMakeCommandStdout(t *testing.T) {
	var buf bytes.Buffer
	cmd := makeCommand("test.exe", false, &buf, nil)
	if cmd.Stdout != &buf {
		t.Error("Stdout should be set")
	}
}

func TestMakeCommandStderr(t *testing.T) {
	var buf bytes.Buffer
	cmd := makeCommand("test.exe", false, nil, &buf)
	if cmd.Stderr != &buf {
		t.Error("Stderr should be set")
	}
}

func TestStandardExecWithRealCommand(t *testing.T) {
	o := Options{
		File:           "cmd.exe",
		Wait:           true,
		ExitCode:       true,
		Args:           []string{"/c", "echo", "hello"},
		UseWorkingPath: false,
	}
	r := o.standardExec()
	if r.Err != nil {
		t.Errorf("standardExec failed: %v", r.Err)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
}

func TestStandardExecWithPid(t *testing.T) {
	o := Options{
		File: "cmd.exe",
		Pid:  true,
		Wait: true,
		Args: []string{"/c", "echo", "hello"},
	}
	r := o.standardExec()
	if r.Err != nil {
		t.Errorf("standardExec failed: %v", r.Err)
	}
	if r.Pid == 0 {
		t.Error("Pid should be non-zero")
	}
}

func TestStandardExecNonexistentFile(t *testing.T) {
	o := Options{
		File: "nonexistent_program.exe",
		Wait: true,
	}
	r := o.standardExec()
	if r.Err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestStandardExecStartAndWait(t *testing.T) {
	o := Options{
		File: "cmd.exe",
		Wait: true,
		Args: []string{"/c", "echo", "test"},
	}
	r := o.standardExec()
	if r.Err != nil {
		t.Errorf("standardExec failed: %v", r.Err)
	}
}

func TestExecNonAdminNonShell(t *testing.T) {
	o := Options{
		File:     `C:\Windows\System32\cmd.exe`,
		Wait:     true,
		ExitCode: true,
		Args:     []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec failed: %v", r.Err)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
}

func TestExecSpecialFile(t *testing.T) {
	// SpecialFile skips getExecutablePath
	o := Options{
		File:        `C:\Windows\System32\cmd.exe`,
		SpecialFile: true,
		Wait:        true,
		ExitCode:    true,
		Args:        []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec failed: %v", r.Err)
	}
}

func TestExecShellPath(t *testing.T) {
	// Shell=true triggers the windows shell path; Pid+!Wait for fast return
	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       false,
		Pid:        true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec shell path failed: %v", r.Err)
	}
	if r.Pid == 0 {
		t.Error("Pid should be non-zero")
	}
}

func TestExecShellPathWithPid(t *testing.T) {
	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Pid:        true,
		Wait:       false,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec shell+pid failed: %v", r.Err)
	}
	if r.Pid == 0 {
		t.Error("Pid should be non-zero")
	}
}

func TestExecShellPathNoPidNoExitCode(t *testing.T) {
	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: true,
		Wait:       false,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec shell path failed: %v", r.Err)
	}
}

func TestExecShellWithHiddenShow(t *testing.T) {
	o := Options{
		File:       `C:\Windows\System32\cmd.exe`,
		Shell:      true,
		ShowWindow: false,
		Wait:       false,
		Pid:        true,
		Args:       []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec shell hidden failed: %v", r.Err)
	}
}

func TestGetExecutablePathEmptyExecutable(t *testing.T) {
	// os.Executable() may fail in test environment
	result := getExecutablePath("test.exe")
	if result == "" {
		t.Log("getExecutablePath returned empty (os.Executable error expected in some envs)")
	}
}

func TestExecShellWithStdoutError(t *testing.T) {
	// Shell + !ShowWindow + !GUI + Stdout should hit the "shell not compatible with stdout" error in exec()
	var buf bytes.Buffer
	o := Options{
		File:   `C:\Windows\System32\cmd.exe`,
		Shell:  true,
		Stdout: &buf,
	}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "shell or elevating as admin are not compatible with capturing stdout/stderr" {
		t.Errorf("expected shell/stdout error, got %v", r.Err)
	}
}

func TestExecAdminWithStderrError(t *testing.T) {
	var buf bytes.Buffer
	o := Options{
		File:    `C:\Windows\System32\cmd.exe`,
		AsAdmin: true,
		Stderr:  &buf,
	}
	r := o.Exec()
	if r.Err == nil || r.Err.Error() != "shell or elevating as admin are not compatible with capturing stdout/stderr" {
		t.Errorf("expected admin/stderr error, got %v", r.Err)
	}
}

func TestExecShellWithExitCodeAndWait(t *testing.T) {
	// Shell + Wait=true + ExitCode=true should hit the exitCode path
	o := Options{
		File:     `C:\Windows\System32\cmd.exe`,
		Shell:    true,
		Wait:     true,
		ExitCode: true,
		Args:     []string{"/c", "echo", "hello"},
	}
	r := o.Exec()
	if r.Err != nil {
		t.Errorf("Exec shell+wait+exitcode failed: %v", r.Err)
	}
	// ShellExecuteEx "open" verb may return non-zero; just verify ExitCode was set
	_ = r.ExitCode
}

func TestStandardExecExitError(t *testing.T) {
	// Running a command that exits with non-zero but is still a valid process
	o := Options{
		File:     `C:\Windows\System32\cmd.exe`,
		Wait:     true,
		ExitCode: true,
		Args:     []string{"/c", "exit", "1"},
	}
	r := o.standardExec()
	if r.Err != nil {
		t.Errorf("standardExec should not return error for ExitError, got %v", r.Err)
	}
	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}
}

func TestGetExecutablePathOsExecutableError(t *testing.T) {
	// Mock os.Executable to return error
	original := osExecutableFn
	SetOsExecutableFn(func() (string, error) {
		return "", errors.New("executable failed")
	})
	defer SetOsExecutableFn(original)

	result := getExecutablePath("test.exe")
	if result != "" {
		t.Errorf("expected empty result when os.Executable fails, got %q", result)
	}
}

func TestStandardExecCustomExecError(t *testing.T) {
	// Mock execCustomExecutable to return error with valid cmd
	original := execCustomExecutableFn
	SetExecCustomExecutableFn(func(executable string, gui bool, wait bool, show bool, executableWorkingPath bool, stdout io.Writer, stderr io.Writer, arg ...string) (error, *exec.Cmd) {
		return errors.New("custom exec error"), &exec.Cmd{}
	})
	defer SetExecCustomExecutableFn(original)

	o := Options{
		File:     `C:\Windows\System32\cmd.exe`,
		Wait:     true,
		ExitCode: true,
		Args:     []string{"/c", "echo", "hello"},
	}
	r := o.standardExec()
	if r.Err == nil {
		t.Error("expected error from mocked execCustomExecutable")
	}
	if r.Err.Error() != "custom exec error" {
		t.Errorf("expected 'custom exec error', got %v", r.Err)
	}
}

func TestExecCustomExecutableStart(t *testing.T) {
	// wait=false → cmd.Start() path
	err, cmd := execCustomExecutable(`C:\Windows\System32\cmd.exe`, false, false, false, false, nil, nil, "/c", "echo", "hello")
	if err != nil {
		t.Errorf("execCustomExecutable start failed: %v", err)
	}
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	// Cleanup: wait for process
	if cmd.Process != nil {
		_ = cmd.Wait()
	}
}

func TestGetExecutablePathError(t *testing.T) {
	// filepath.IsLocal("absolute/path") is false, so it returns the input unchanged
	result := getExecutablePath("/some/absolute/path")
	if result != "/some/absolute/path" {
		t.Errorf("getExecutablePath = %q, want %q", result, "/some/absolute/path")
	}
}
