package watch

import (
	"context"
	"os"
	"sort"
	"time"

	commonProcess "github.com/luskaner/ageServer/common/process"
)

// sortedProcessNames returns the map keys in a deterministic order so log
// output and waiting order do not depend on Go's randomized map iteration.
func sortedProcessNames(processes map[string]*os.Process) []string {
	names := make([]string, 0, len(processes))
	for name := range processes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// waitForProcessesToExit waits for every process to exit. It used to wait for
// a single randomly-chosen process: if that one exited first while the game
// was still running, the agent reverted the configuration too early.
//
// The wait is cancellable because there is no telling in advance how long a game
// session lasts, so this is a wait for as long as it takes, and the user asking
// the agent to stop is the only thing that can reasonably end it early. A false
// result therefore means one of two things, and the caller has to say which.
func waitForProcessesToExit(ctx context.Context, processes []*os.Process) bool {
	allExited := true
	for _, p := range processes {
		if p == nil {
			continue
		}
		if !commonProcess.WaitForProcessContext(ctx, p, nil) {
			allExited = false
		}
	}
	return allExited
}

// waitUntilAnyProcessExist polls for the first of names to appear, and gives up
// after timeout.
//
// Both the giving up and the giving in are the caller's business, so the timeout
// is a parameter rather than the minute it used to be hard-coded to: the battle
// server wait inside the rebroadcast has always been a different one, and a
// caller that was stopped mid-wait should be able to tell so.
func waitUntilAnyProcessExist(ctx context.Context, names []string, interval time.Duration, timeout time.Duration) (processes map[string]*os.Process) {
	deadline := time.Now().Add(timeout)
	for {
		if processes = commonProcessProcessesByNamesFn(names); len(processes) > 0 {
			return
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}
