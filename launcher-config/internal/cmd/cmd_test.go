package cmd

import (
	"testing"

	"github.com/luskaner/ageServer/common"
)

func resetCmdState(t *testing.T) {
	t.Helper()
	oldSetupValues := setupValues
	oldRevertValues := revertValues
	oldPath := path
	t.Cleanup(func() {
		setupValues = oldSetupValues
		revertValues = oldRevertValues
		path = oldPath
	})
	setupValues = nil
	revertValues = nil
	path = nil
}

func TestRunSetUpSyntaxError(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runSetUp([]string{"--this-flag-does-not-exist"})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunSetUpMissingGameFlag(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runSetUp([]string{})

	if err == nil {
		t.Fatal("expected error for missing --game flag")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunRevertSyntaxError(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runRevert([]string{"--this-flag-does-not-exist"})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunRevertMissingGameFlag(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runRevert([]string{})

	if err == nil {
		t.Fatal("expected error for missing --game flag")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunFlushCacheSyntaxError(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runFlushCache([]string{"--this-flag-does-not-exist"})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunFlushCacheNoFlagsReturnsNoError(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runFlushCache([]string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunStopAgentNoError(t *testing.T) {
	resetCmdState(t)

	err, exitCode := runStopAgent([]string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}
