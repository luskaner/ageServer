//go:build !windows && !darwin && !linux

// This file provides fallback implementations for unsupported Unix systems.
// It implements GetProcessStartTime, WaitForProcess, and ProcessesPID with
// minimal functionality to allow the package to compile and run on any Unix system.

package process

import (
	"context"
	"os"
	"time"
)

func GetProcessStartTime(_ int) (int64, error) {
	// Fallback for unsupported Unix systems - always return 0
	// This disables startTime validation but maintains basic functionality
	return 0, nil
}

// WaitForProcessContext is the fallback: this platform cannot tell, so it
// reports that the wait succeeded, which is what WaitForProcess has always done
// here.
func WaitForProcessContext(_ context.Context, _ *os.Process, _ *time.Duration) bool {
	return true
}

// WaitForProcess waits for proc to exit, and reports whether it did.
func WaitForProcess(proc *os.Process, duration *time.Duration) bool {
	return WaitForProcessContext(context.Background(), proc, duration)
}

// ProcessesByNames returns a map of process names to their procs.
// Note: If multiple processes share the same name, only one PID is stored per name.
func ProcessesByNames(_ []string) map[string]*os.Process {
	return make(map[string]*os.Process)
}
