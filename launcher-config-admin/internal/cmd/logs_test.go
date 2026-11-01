package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func resetLogState(t *testing.T) {
	t.Helper()
	oldLogger := internal.Logger
	internal.Logger = nil
	t.Cleanup(func() { internal.Logger = oldLogger })
}

// Regression: runSetUp did not return after a flag parse error, so it kept
// executing the privileged setup and initialized file logging (creating the
// log directory) even for invalid invocations.
func TestRunSetUpSyntaxErrorStopsBeforeSideEffects(t *testing.T) {
	resetLogState(t)
	logRoot := filepath.Join(t.TempDir(), "logs")

	err, exitCode := runSetUp([]string{
		"--game", "age2",
		"--logRoot", logRoot,
		"--this-flag-does-not-exist",
	})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
	if internal.Logger != nil {
		t.Fatal("file logging must NOT be initialized on syntax error")
	}
	if _, statErr := os.Stat(logRoot); !os.IsNotExist(statErr) {
		t.Fatal("log root directory must not be created on syntax error")
	}
}

func TestRunRevertSyntaxErrorReturnsImmediately(t *testing.T) {
	resetLogState(t)
	logRoot := filepath.Join(t.TempDir(), "logs")

	err, exitCode := runRevert([]string{
		"--logRoot", logRoot,
		"--this-flag-does-not-exist",
	})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
	if internal.Logger != nil {
		t.Fatal("file logging must NOT be initialized on syntax error")
	}
	if _, statErr := os.Stat(logRoot); !os.IsNotExist(statErr) {
		t.Fatal("log root directory must not be created on syntax error")
	}
}

func TestRunSetUpMissingGameFlagReturnsError(t *testing.T) {
	resetLogState(t)

	err, exitCode := runSetUp([]string{})

	if err == nil {
		t.Fatal("expected error for missing --game flag")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
}

func TestRunFlushCacheSyntaxErrorReturnsImmediately(t *testing.T) {
	resetLogState(t)
	logRoot := filepath.Join(t.TempDir(), "logs")

	err, exitCode := runFlushCache([]string{
		"--logRoot", logRoot,
		"--this-flag-does-not-exist",
	})

	if err == nil {
		t.Fatal("expected syntax error")
	}
	if exitCode != common.ErrSyntax {
		t.Fatalf("exitCode = %d, want ErrSyntax", exitCode)
	}
	if internal.Logger != nil {
		t.Fatal("file logging must NOT be initialized on syntax error")
	}
}

func TestRunFlushCacheNoFlagsReturnsNoError(t *testing.T) {
	resetLogState(t)

	err, exitCode := runFlushCache([]string{})

	// No flags means no IPs/Certs to flush, should succeed without error
	if err != nil {
		t.Fatalf("unexpected error: %v", exitCode)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunRevertRemoveAllSetsBothFlags(t *testing.T) {
	resetLogState(t)

	// --all should set both IPs and Certs to true
	// We can't fully run it (OS-level), but we verify the flag is accepted
	// by checking it doesn't syntax-error
	err, exitCode := runRevert([]string{"--all", "--logRoot", filepath.Join(t.TempDir(), "logs")})

	// It will fail later (OS-level cert/hosts operations), but shouldn't be a syntax error
	if exitCode == common.ErrSyntax {
		t.Fatalf("all flag caused syntax error: %v", err)
	}
}
