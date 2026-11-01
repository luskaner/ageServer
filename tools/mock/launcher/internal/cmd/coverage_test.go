package cmd

import (
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/executor/exec"
)

func TestRootCmdExecSuccess(t *testing.T) {
	oldExec := execFn
	oldExit, oldWait, oldExe := exitBeforeRunning, waitBeforeRunning, executableGame
	defer func() {
		execFn = oldExec
		exitBeforeRunning, waitBeforeRunning, executableGame = oldExit, oldWait, oldExe
	}()
	exitBeforeRunning = false
	waitBeforeRunning = 0
	executableGame = "game.exe"
	execFn = func(options exec.Options) *exec.Result {
		if options.File != "game.exe" {
			t.Fatalf("file %q", options.File)
		}
		return &exec.Result{Pid: 1234, ExitCode: 0}
	}
	if err := rootCmd([]string{"arg1"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestRootCmdExecFailure(t *testing.T) {
	oldExec := execFn
	oldExit, oldWait, oldExe := exitBeforeRunning, waitBeforeRunning, executableGame
	defer func() {
		execFn = oldExec
		exitBeforeRunning, waitBeforeRunning, executableGame = oldExit, oldWait, oldExe
	}()
	exitBeforeRunning = false
	waitBeforeRunning = 0
	executableGame = "game.exe"
	execFn = func(exec.Options) *exec.Result {
		return &exec.Result{Err: errors.New("fail"), ExitCode: 1}
	}
	if err := rootCmd(nil); err == nil {
		t.Fatal("should fail")
	}
}

func TestRootCmdWait(t *testing.T) {
	oldExec := execFn
	oldWait := waitBeforeRunning
	defer func() { execFn = oldExec; waitBeforeRunning = oldWait }()
	waitBeforeRunning = 10 * time.Millisecond
	exitBeforeRunning = false
	executableGame = "game.exe"
	execFn = func(exec.Options) *exec.Result { return &exec.Result{Pid: 1} }
	start := time.Now()
	rootCmd(nil)
	if time.Since(start) < 5*time.Millisecond {
		t.Fatal("should have waited")
	}
}

func TestExecute(t *testing.T) {
	oldExec := execFn
	defer func() { execFn = oldExec }()
	execFn = func(exec.Options) *exec.Result { return &exec.Result{Pid: 1} }
	setupFlags()
	if err := flag.Set("exitBeforeRunning", "true"); err != nil {
		t.Fatal(err)
	}
	if err := flag.Set("waitBeforeRunning", "0s"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		flag.Set("exitBeforeRunning", "false")
		flag.Set("waitBeforeRunning", "10s")
	}()
	if err := Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}
