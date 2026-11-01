package process

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// A cancelled context must stop the wait, not merely shorten it.
//
// This is what a stop button in a graphical launcher needs: the longest wait in
// the launcher is for a previous instance to go away, and there is nothing a user
// can do about a minute of silence if the wait cannot be cut short.
func TestWaitForProcessContextCancelsInsteadOfWaitingOutTheTimeout(t *testing.T) {
	proc := startSleepingChild(t)

	// A timeout long enough that finishing it would mean the test passed by
	// accident rather than by cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	long := time.Minute
	if WaitForProcessContext(ctx, proc, &long) {
		t.Error("a cancelled wait reported that the process exited")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("the wait took %s, so it ran out the timeout instead of noticing the cancellation", elapsed)
	}
}

// An already cancelled context must not wait at all.
func TestWaitForProcessContextAlreadyCancelled(t *testing.T) {
	proc := startSleepingChild(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if WaitForProcessContext(ctx, proc, nil) {
		t.Error("a cancelled wait reported that the process exited")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("an already cancelled wait took %s", elapsed)
	}
}

// Cancelling must not change what an uncancelled wait reports, or the launcher
// would start tearing down a run that was going to succeed.
func TestWaitForProcessContextUnchangedWithoutCancellation(t *testing.T) {
	proc := startSleepingChild(t)
	d := 10 * time.Second
	if WaitForProcessContext(context.Background(), proc, &d) {
		t.Error("a live process with time left reported that it exited")
	}
}

// The same, with no timeout at all.
//
// This is the wait the agent's stop button is built on: there is no telling in
// advance how long a game session lasts, so it is asked to wait "for as long as
// it takes", and the only thing that can end that wait is the caller changing
// its mind. Windows reached for WaitForSingleObject(INFINITE) whenever the
// context had no deadline, which every cancellable context does not, so the
// cancellation was never noticed and the agent sat there for the rest of the
// game.
func TestWaitForProcessContextCancelsAnUntimedWait(t *testing.T) {
	proc := startSleepingChild(t)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if WaitForProcessContext(ctx, proc, nil) {
		t.Error("a cancelled wait reported that the process exited")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("an untimed wait took %s, so it never noticed the cancellation", elapsed)
	}
}

// The version without a context is the one everything else calls, and it must
// keep behaving exactly as before.
func TestWaitForProcessStillTimesOut(t *testing.T) {
	proc := startSleepingChild(t)
	d := 100 * time.Millisecond
	if WaitForProcess(proc, &d) {
		t.Error("a live process with a short timeout reported that it exited")
	}
}

// startSleepingChild returns a process that will still be running when the test
// ends, so the wait has something real to wait for.
func startSleepingChild(t *testing.T) *os.Process {
	t.Helper()
	// The child is the test binary itself, told to do nothing: starting a shell
	// that may not exist, or a platform-specific sleeper, would make this test
	// about the platform rather than about the wait.
	cmd := exec.Command(os.Args[0], "-test.run", "TestHelperProcessThatBlocks")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the child: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd.Process
}

// TestHelperProcessThatBlocks is the child. It sleeps until it is killed, so the
// parent always has a process that outlives a short wait.
func TestHelperProcessThatBlocks(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("not the helper process")
	}
	time.Sleep(30 * time.Second)
}
