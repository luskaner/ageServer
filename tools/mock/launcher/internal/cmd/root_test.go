package cmd

import (
	"flag"
	"testing"
)

// Regression: exitBeforeRunning existed as a variable but was never
// registered as a flag, making the feature unreachable.
func TestSetupFlagsRegistersExitBeforeRunning(t *testing.T) {
	setupFlags()
	if f := flag.Lookup("exitBeforeRunning"); f == nil {
		t.Fatal("-exitBeforeRunning flag not registered")
	} else if f.DefValue != "false" {
		t.Fatalf("default = %q, want false", f.DefValue)
	}
	if flag.Lookup("waitBeforeRunning") == nil {
		t.Fatal("-waitBeforeRunning flag not registered")
	}
}

func TestRootCmdExitsBeforeRunning(t *testing.T) {
	oldExit, oldWait, oldExe := exitBeforeRunning, waitBeforeRunning, executableGame
	exitBeforeRunning = true
	waitBeforeRunning = 0
	executableGame = "should-not-run.exe"
	defer func() {
		exitBeforeRunning, waitBeforeRunning, executableGame = oldExit, oldWait, oldExe
	}()

	// With exitBeforeRunning set, rootCmd must return before attempting to
	// launch executableGame ("should-not-run.exe" would fail to exec).
	if err := rootCmd(nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
