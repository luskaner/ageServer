package internal

import (
	"errors"
	"testing"

	commonLogger "github.com/luskaner/ageServer/common/logger"
)

func TestInitializeOrExitSuccess(t *testing.T) {
	oldFn := newOwnFileLoggerFn
	oldExit := osExitFn
	oldLogger := commonLogger.FileLogger
	t.Cleanup(func() {
		newOwnFileLoggerFn = oldFn
		osExitFn = oldExit
		commonLogger.FileLogger = oldLogger
	})
	commonLogger.FileLogger = nil
	newOwnFileLoggerFn = func(string, string, string, bool) error { return nil }
	calledExit := false
	osExitFn = func(int) { calledExit = true }
	InitializeOrExit("/tmp")
	if calledExit {
		t.Fatal("should not exit on success")
	}
}

func TestInitializeOrExitFailureExits(t *testing.T) {
	oldFn := newOwnFileLoggerFn
	oldExit := osExitFn
	t.Cleanup(func() {
		newOwnFileLoggerFn = oldFn
		osExitFn = oldExit
	})
	newOwnFileLoggerFn = func(string, string, string, bool) error { return errors.New("fail") }
	exitCode := -1
	osExitFn = func(c int) { exitCode = c }
	InitializeOrExit("/bad")
	if exitCode == -1 {
		t.Fatal("should have called osExit")
	}
}
