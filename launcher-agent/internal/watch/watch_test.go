package watch

import (
	"context"
	"errors"
	"io"
	"os"
	gos_exec "os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-agent/internal"
	"github.com/luskaner/ageServer/launcher-common/cmd/agent"
)

// Helper process: re-executed by the tests to create real child processes
// that sleep for SLEEP_MS milliseconds and exit successfully.
func TestHelperProcessSleep(t *testing.T) {
	if os.Getenv("GO_WATCH_HELPER") != "1" {
		return
	}
	ms, _ := strconv.Atoi(os.Getenv("SLEEP_MS"))
	time.Sleep(time.Duration(ms) * time.Millisecond)
	os.Exit(0)
}

func spawnSleeper(t *testing.T, ms int) *os.Process {
	t.Helper()
	cmd := gos_exec.Command(os.Args[0], "-test.run=TestHelperProcessSleep$", "-test.v")
	cmd.Env = append(os.Environ(), "GO_WATCH_HELPER=1", "SLEEP_MS="+strconv.Itoa(ms))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd.Process
}

func TestWaitForProcessesToExitWaitsForAll(t *testing.T) {
	fast := spawnSleeper(t, 50)
	slow := spawnSleeper(t, 500)

	start := time.Now()
	if !waitForProcessesToExit(context.Background(), []*os.Process{fast, slow}) {
		t.Fatal("expected successful wait")
	}
	elapsed := time.Since(start)
	if elapsed < 400*time.Millisecond {
		t.Fatalf("returned after %v; it did not wait for ALL processes (regression: single random pick)", elapsed)
	}
}

// The wait is for however long the game lasts, so it has to be the one thing
// that can end it: without this the tray's Exit would take the icon away and
// leave the agent waiting for a game the user has given up on.
func TestWaitForProcessesToExitGivesUpWhenCancelled(t *testing.T) {
	never := spawnSleeper(t, 60000)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if waitForProcessesToExit(ctx, []*os.Process{never}) {
		t.Error("a cancelled wait reported that the process exited")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("the wait took %v, so it never noticed the cancellation", elapsed)
	}
}

func TestWaitForProcessesToExitEmptyAndNil(t *testing.T) {
	if !waitForProcessesToExit(context.Background(), nil) {
		t.Fatal("empty list must succeed")
	}
	if !waitForProcessesToExit(context.Background(), []*os.Process{nil}) {
		t.Fatal("nil entries must be skipped")
	}
}

// The tray's second row has to name what the agent is watching, in a form that
// can be acted on: a game on its own does not tell anybody which process to look
// for, and a bare list of PIDs does not say whose they are.
func TestSessionNamesTheGameAndItsProcesses(t *testing.T) {
	got := session("age2", []string{"aoe2.exe", "helper.exe"}, []int{11, 22})
	if got != "age2 — aoe2.exe (pid 11), helper.exe (pid 22)" {
		t.Errorf("session = %q", got)
	}
}

// The processes are not known until the game is found, and an unknown game is
// still worth naming rather than dropping the row's whole content.
func TestSessionWithoutAGameOrWithoutProcesses(t *testing.T) {
	if got := session("", nil, nil); got != "no process" {
		t.Errorf("session with nothing known = %q", got)
	}
	if got := session("athens", nil, nil); got != "athens — no process" {
		t.Errorf("session with only a game = %q", got)
	}
}

func TestSortedProcessNames(t *testing.T) {
	processes := map[string]*os.Process{
		"zebra.exe": nil,
		"alpha.exe": nil,
		"mid.exe":   nil,
	}
	names := sortedProcessNames(processes)
	want := []string{"alpha.exe", "mid.exe", "zebra.exe"}
	if len(names) != len(want) {
		t.Fatalf("names = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}

// Regression: the agent is frequently started without a log root (no
// --logRoot/--baseDataPath), which leaves internal.Logger nil. loggerBufferFn
// wraps the configuration revert (config_revert_end) and the battle-server
// removal, so it must still run that work when there is no file to log to.
// Returning early here used to skip the revert entirely and leave the hosts
// file, the certificate and the battle server in place after a session.
func TestLoggerBufferFnRunsWorkWhenLoggerNil(t *testing.T) {
	orig := internal.Logger
	internal.Logger = nil
	t.Cleanup(func() { internal.Logger = orig })

	called := false
	if err := loggerBufferFn("config_revert_end", func(w io.Writer) {
		called = true
		if w != nil {
			t.Errorf("expected a nil writer when there is no file logger, got %v", w)
		}
	}); err != nil {
		t.Fatalf("loggerBufferFn returned error: %v", err)
	}
	if !called {
		t.Fatal("loggerBufferFn must run the wrapped work (the config revert) even when internal.Logger is nil")
	}
}

func testValues() *agent.Values {
	return &agent.Values{
		GameIdValues:  &cmd.GameIdValues{GameId: "age2"},
		LogRootValues: &cmd.LogRootValues{LogRoot: ""},
	}
}

type watchOverrides struct {
	waitUntilAnyProcessExistFnVal func(context.Context, []string, time.Duration, time.Duration) map[string]*os.Process
	waitForProcessesToExitFnVal   func(context.Context, []*os.Process) bool
	processesByNamesFnVal         func([]string) map[string]*os.Process
	serverKillDoFnVal             func(string) error
	configRevertFnVal             func(string, string, bool, io.Writer, func(*exec.Options), func([]string, bool, io.Writer, func(*exec.Options)) *exec.Result) bool
	runRevertCommandFnVal         func(io.Writer, func(*exec.Options)) error
	removeBattleServerRegionFnVal func(string, string, string, io.Writer, func(*exec.Options)) *exec.Result
	loggerBufferFnVal             func(string, func(io.Writer)) error
	gameLogsCopyFnVal             func(string, string, string)
	rebroadcastFnVal              func(context.Context, *ExitCode, int)
}

// watchOverrides applies a set of replacements and returns the function that puts
// the originals back.
//
// The signature each one has to match grew a context when the waits became
// cancellable, which is why there are two blocks here rather than one list: the
// call sites read the signature, not this.
func applyWatchOverrides(o watchOverrides) func() {
	origProcessFn := waitUntilAnyProcessExistFn
	origWaitFn := waitForProcessesToExitFn
	origByNamesFn := commonProcessProcessesByNamesFn
	origKillFn := serverKillDoFn
	origConfigRevert := configRevertFn
	origRevertCommand := runRevertCommandFn
	origRemoveBSRegion := removeBattleServerRegionFn
	origBufferFn := loggerBufferFn
	origCopyLogs := gameLogsCopyFn
	origRebroadcast := rebroadcastFn

	if o.waitUntilAnyProcessExistFnVal != nil {
		waitUntilAnyProcessExistFn = o.waitUntilAnyProcessExistFnVal
	}
	if o.waitForProcessesToExitFnVal != nil {
		waitForProcessesToExitFn = o.waitForProcessesToExitFnVal
	}
	if o.processesByNamesFnVal != nil {
		commonProcessProcessesByNamesFn = o.processesByNamesFnVal
	}
	if o.serverKillDoFnVal != nil {
		serverKillDoFn = o.serverKillDoFnVal
	}
	if o.configRevertFnVal != nil {
		configRevertFn = o.configRevertFnVal
	}
	if o.runRevertCommandFnVal != nil {
		runRevertCommandFn = o.runRevertCommandFnVal
	}
	if o.removeBattleServerRegionFnVal != nil {
		removeBattleServerRegionFn = o.removeBattleServerRegionFnVal
	}
	if o.loggerBufferFnVal != nil {
		loggerBufferFn = o.loggerBufferFnVal
	}
	if o.gameLogsCopyFnVal != nil {
		gameLogsCopyFn = o.gameLogsCopyFnVal
	}
	if o.rebroadcastFnVal != nil {
		rebroadcastFn = o.rebroadcastFnVal
	}

	return func() {
		waitUntilAnyProcessExistFn = origProcessFn
		waitForProcessesToExitFn = origWaitFn
		commonProcessProcessesByNamesFn = origByNamesFn
		serverKillDoFn = origKillFn
		configRevertFn = origConfigRevert
		runRevertCommandFn = origRevertCommand
		removeBattleServerRegionFn = origRemoveBSRegion
		loggerBufferFn = origBufferFn
		gameLogsCopyFn = origCopyLogs
		rebroadcastFn = origRebroadcast
	}
}

func noopConfigRevert(gameId, logRoot string, headless bool, out io.Writer, optionsFn func(*exec.Options), runRevertFn func([]string, bool, io.Writer, func(*exec.Options)) *exec.Result) bool {
	return true
}

func noopRevertCommand(out io.Writer, optionsFn func(*exec.Options)) error {
	return nil
}

func noopLoggerBuffer(name string, fn func(io.Writer)) error {
	return nil
}

// runningLoggerBuffer is noopLoggerBuffer that does the work.
//
// The buffer only decides where the wrapped work is logged, so a test that wants
// to know whether the teardown reached the revert has to let it run. The other
// tests use the no-op because they only care about the path taken to it.
func runningLoggerBuffer(name string, fn func(io.Writer)) error {
	fn(nil)
	return nil
}

func noopRemoveBSRegion(exe, gameId, region string, out io.Writer, optionsFn func(*exec.Options)) *exec.Result {
	return &exec.Result{}
}

func TestWatchGameTimeoutStart(t *testing.T) {
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return nil
		},
		serverKillDoFnVal:     func(name string) error { return nil },
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = ""
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if exitCode.Get() != internal.ErrGameTimeoutStart {
		t.Errorf("expected %d for game timeout, got %d", internal.ErrGameTimeoutStart, exitCode.Get())
	}
}

func TestWatchGameFound(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return true },
		serverKillDoFnVal:           func(name string) error { return nil },
		configRevertFnVal:           noopConfigRevert,
		runRevertCommandFnVal:       noopRevertCommand,
		loggerBufferFnVal:           noopLoggerBuffer,
		gameLogsCopyFnVal:           func(gameId, basePath, logRoot string) {},
		rebroadcastFnVal:            func(_ context.Context, exitCode *ExitCode, port int) {},
	})()

	values := testValues()
	values.ServerExecutable = ""
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if exitCode.Get() != common.ErrSuccess {
		t.Errorf("expected success, got %d", exitCode.Get())
	}
}

func TestWatchWithServerKill(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	killCalled := false
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return true },
		serverKillDoFnVal: func(name string) error {
			killCalled = true
			return nil
		},
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = "server.exe"
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if !killCalled {
		t.Error("expected serverKill to be called")
	}
	if exitCode.Get() != common.ErrSuccess {
		t.Errorf("expected success, got %d", exitCode.Get())
	}
}

func TestWatchServerKillFailure(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return true },
		serverKillDoFnVal: func(name string) error {
			return os.ErrProcessDone
		},
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = "server.exe"
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if exitCode.Get() != internal.ErrFailedStopServer {
		t.Errorf("expected %d for kill failure, got %d", internal.ErrFailedStopServer, exitCode.Get())
	}
}

func TestWatchWithLogCopy(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	logCopied := false
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return true },
		serverKillDoFnVal:           func(name string) error { return nil },
		configRevertFnVal:           noopConfigRevert,
		runRevertCommandFnVal:       noopRevertCommand,
		loggerBufferFnVal:           noopLoggerBuffer,
		gameLogsCopyFnVal: func(gameId, basePath, logRoot string) {
			logCopied = true
		},
	})()

	values := testValues()
	values.ServerExecutable = ""
	values.LogRoot = "/tmp/logs"
	values.BaseDataPath = "/tmp/data"
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if !logCopied {
		t.Error("expected game logs to be copied")
	}
}

func TestWatchCleanupOncePreventsDoubleCleanup(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	killCount := 0
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return true },
		serverKillDoFnVal: func(name string) error {
			killCount++
			return nil
		},
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = "server.exe"
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if killCount != 1 {
		t.Errorf("expected serverKill to be called exactly once, got %d", killCount)
	}
}

func TestWatchWaitFailure(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool { return false },
		serverKillDoFnVal:           func(name string) error { return nil },
		configRevertFnVal:           noopConfigRevert,
		runRevertCommandFnVal:       noopRevertCommand,
		loggerBufferFnVal:           noopLoggerBuffer,
		gameLogsCopyFnVal:           func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = ""
	exitCode := NewExitCode()
	var once sync.Once
	Watch(context.Background(), values, exitCode, &once)
	if exitCode.Get() != internal.ErrFailedWaitForProcess {
		t.Errorf("expected %d for wait failure, got %d", internal.ErrFailedWaitForProcess, exitCode.Get())
	}
}

// Regression: Watch (which waits for the game) and the signal handler in
// cmd.runRoot both wrote the exit code through a bare *int, and the cleanup ran
// on whichever goroutine won sync.Once. That is a data race, and it stayed
// invisible on Windows because the agent is started detached with
// CREATE_NO_WINDOW, so it has no console, Ctrl+C never reaches it and SIGTERM
// does not exist: the second writer is only live where the signal can fire.
// Run with -race to be meaningful.
func TestExitCodeIsRaceFree(t *testing.T) {
	exitCode := NewExitCode()
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		for range 500 {
			exitCode.SetIfSuccess(internal.ErrFailedStopServer)
		}
	}()
	go func() {
		defer wg.Done()
		for range 500 {
			_ = exitCode.Get()
		}
	}()
	go func() {
		defer wg.Done()
		exitCode.Set(common.ErrSignal)
	}()
	go func() {
		defer wg.Done()
		for range 500 {
			if exitCode.Get() == common.ErrSuccess {
				exitCode.Set(internal.ErrGameTimeoutStart)
			}
		}
	}()
	wg.Wait()
}

// The first failure must survive: a later step reporting a different error
// cannot mask an earlier one, which is what the old
// `if *exitCode == ErrSuccess { *exitCode = X }` guards were for.
func TestExitCodeFirstFailureWins(t *testing.T) {
	exitCode := NewExitCode()
	if exitCode.Get() != common.ErrSuccess {
		t.Fatalf("a fresh ExitCode should start at ErrSuccess, got %d", exitCode.Get())
	}
	exitCode.SetIfSuccess(internal.ErrFailedStopServer)
	exitCode.SetIfSuccess(internal.ErrBattleServerTimeOutStart)
	if exitCode.Get() != internal.ErrFailedStopServer {
		t.Errorf("expected the first failure %d to win, got %d", internal.ErrFailedStopServer, exitCode.Get())
	}
	// Set is unconditional, so an interrupting signal still takes effect.
	exitCode.Set(common.ErrSignal)
	if exitCode.Get() != common.ErrSignal {
		t.Errorf("expected %d, got %d", common.ErrSignal, exitCode.Get())
	}
}

// Regression: the signal handler interrupts Watch while it is blocked waiting
// for the game, then runs the cleanup itself. Both paths touch the exit code.
func TestWatchAndCleanupShareExitCodeSafely(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	waiting := make(chan struct{})
	resume := make(chan struct{})
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool {
			close(waiting)
			<-resume
			return false
		},
		serverKillDoFnVal:     func(name string) error { return errors.New("kill failed") },
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = "server.exe"
	exitCode := NewExitCode()
	var once sync.Once

	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(context.Background(), values, exitCode, &once)
	}()

	<-waiting
	// The signal handler: record the interrupt, then clean up itself.
	exitCode.Set(common.ErrSignal)
	once.Do(func() { Cleanup(values, exitCode) })
	close(resume)

	<-done
	// The interrupt wins, and the cleanup's later failure does not overwrite it.
	if exitCode.Get() != common.ErrSignal {
		t.Errorf("expected %d to survive the cleanup, got %d", common.ErrSignal, exitCode.Get())
	}
}

// The Exit in the tray has to be a stop, not a failure.
//
// Everything it relies on was decided by the user: the wait is cut short because
// they asked, and the exit code that says so is the caller's to record. What must
// not happen is the wait's own error surviving, because that would have somebody
// who stopped a session deliberately read the logs of one that broke.
func TestWatchCancelledWhileWaitingIsNotAFailure(t *testing.T) {
	fakeProcess := &os.Process{Pid: 99999}
	reverted := 0
	waiting := make(chan struct{})
	release := make(chan struct{})
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": fakeProcess}
		},
		waitForProcessesToExitFnVal: func(ctx context.Context, processes []*os.Process) bool {
			close(waiting)
			// The real wait returns as soon as the context is cancelled; this one
			// holds on until the test lets go of it, so that the cancellation is
			// known to have happened before the false is returned.
			<-release
			return ctx.Err() != nil
		},
		serverKillDoFnVal: func(name string) error { return nil },
		configRevertFnVal: func(string, string, bool, io.Writer, func(*exec.Options), func([]string, bool, io.Writer, func(*exec.Options)) *exec.Result) bool {
			reverted++
			return true
		},
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     runningLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = "server.exe"
	exitCode := NewExitCode()
	var once sync.Once

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(ctx, values, exitCode, &once)
	}()

	<-waiting
	cancel()
	close(release)
	<-done

	if exitCode.Get() != common.ErrSuccess {
		t.Errorf("a stopped run reported %d; the wait it cut short is not a failure", exitCode.Get())
	}
	// The teardown is what makes stopping worth doing, so it must still have
	// happened, once.
	if reverted != 1 {
		t.Errorf("the configuration was reverted %d times, want exactly 1", reverted)
	}
}

// The same, one step earlier: the minute spent looking for a game that never
// starts is the window where a user who launched the wrong thing reaches for the
// tray's Exit, so it has to be interruptible there too.
func TestWatchCancelledBeforeTheGameStartsIsNotATimeout(t *testing.T) {
	release := make(chan struct{})
	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(ctx context.Context, names []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			<-release
			return nil
		},
		serverKillDoFnVal:     func(name string) error { return nil },
		configRevertFnVal:     noopConfigRevert,
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     noopLoggerBuffer,
		gameLogsCopyFnVal:     func(gameId, basePath, logRoot string) {},
	})()

	values := testValues()
	values.ServerExecutable = ""
	exitCode := NewExitCode()
	var once sync.Once

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(ctx, values, exitCode, &once)
	}()

	cancel()
	close(release)
	<-done

	if exitCode.Get() == internal.ErrGameTimeoutStart {
		t.Error("a run the user stopped was reported as a game that never started")
	}
	if exitCode.Get() != common.ErrSuccess {
		t.Errorf("expected success, got %d", exitCode.Get())
	}
}

func TestWaitUntilAnyProcessExistFound(t *testing.T) {
	origFn := commonProcessProcessesByNamesFn
	defer func() { commonProcessProcessesByNamesFn = origFn }()

	fakeProcess := &os.Process{Pid: 12345}
	commonProcessProcessesByNamesFn = func(names []string) map[string]*os.Process {
		return map[string]*os.Process{"game.exe": fakeProcess}
	}

	processes := waitUntilAnyProcessExist(context.Background(), []string{"game.exe"}, time.Millisecond, time.Minute)
	if len(processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(processes))
	}
	if _, ok := processes["game.exe"]; !ok {
		t.Fatal("expected game.exe in results")
	}
}

func TestWaitUntilAnyProcessExistTimeout(t *testing.T) {
	origFn := commonProcessProcessesByNamesFn
	defer func() { commonProcessProcessesByNamesFn = origFn }()

	commonProcessProcessesByNamesFn = func(names []string) map[string]*os.Process {
		return nil
	}

	start := time.Now()
	processes := waitUntilAnyProcessExist(context.Background(), []string{"nonexistent.exe"}, 50*time.Millisecond, 200*time.Millisecond)
	elapsed := time.Since(start)
	if len(processes) != 0 {
		t.Fatalf("expected empty result, got %d processes", len(processes))
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("expected at least 150ms wait, got %v", elapsed)
	}
}

// The up to a minute spent looking for a game that may never start is a wait
// like any other, and stopping the agent has to be able to end it. Otherwise
// Exit does nothing at all for the whole of the window in which the user is most
// likely to decide they have launched the wrong thing.
func TestWaitUntilAnyProcessExistGivesUpWhenCancelled(t *testing.T) {
	origFn := commonProcessProcessesByNamesFn
	defer func() { commonProcessProcessesByNamesFn = origFn }()

	commonProcessProcessesByNamesFn = func(names []string) map[string]*os.Process {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	processes := waitUntilAnyProcessExist(ctx, []string{"nonexistent.exe"}, 50*time.Millisecond, time.Hour)
	elapsed := time.Since(start)
	if len(processes) != 0 {
		t.Fatalf("expected empty result, got %d processes", len(processes))
	}
	if elapsed > 20*time.Second {
		t.Errorf("a cancelled search took %v, so it ran out the timeout instead", elapsed)
	}
}

// A Steam title starts through a bootstrap that spawns the game and exits, and
// the game is a new PID under one of the watched names. The session is the game,
// not the bootstrap: ending it at the first exit reverted the configuration with
// the game still open, which is what the 2026-10-27 age3 session shows.
func TestWatchKeepsWaitingWhenTheGameHandsOffToANewProcess(t *testing.T) {
	bootstrap := &os.Process{Pid: 4242}
	game := &os.Process{Pid: 5353}
	var waited []int
	scans := 0
	reverted := 0

	defer applyWatchOverrides(watchOverrides{
		waitUntilAnyProcessExistFnVal: func(_ context.Context, _ []string, _ time.Duration, _ time.Duration) map[string]*os.Process {
			return map[string]*os.Process{"game.exe": bootstrap}
		},
		waitForProcessesToExitFnVal: func(_ context.Context, processes []*os.Process) bool {
			for _, p := range processes {
				waited = append(waited, p.Pid)
			}
			return true
		},
		processesByNamesFnVal: func(_ []string) map[string]*os.Process {
			scans++
			if scans == 1 {
				return map[string]*os.Process{"game.exe": game}
			}
			return nil
		},
		serverKillDoFnVal: func(string) error { return nil },
		configRevertFnVal: func(string, string, bool, io.Writer, func(*exec.Options), func([]string, bool, io.Writer, func(*exec.Options)) *exec.Result) bool {
			reverted++
			return true
		},
		runRevertCommandFnVal: noopRevertCommand,
		loggerBufferFnVal:     runningLoggerBuffer,
		gameLogsCopyFnVal:     func(string, string, string) {},
	})()

	values := testValues()
	values.ServerExecutable = ""
	values.ProcessNames = []string{"game.exe"}
	exitCode := NewExitCode()
	var once sync.Once

	Watch(context.Background(), values, exitCode, &once)

	want := []int{bootstrap.Pid, game.Pid}
	if len(waited) != len(want) || waited[0] != want[0] || waited[1] != want[1] {
		t.Fatalf("waited for PIDs %v, want %v: the session ended with the game still running", waited, want)
	}
	if exitCode.Get() != common.ErrSuccess {
		t.Errorf("exit code = %d, want success", exitCode.Get())
	}
	if reverted != 1 {
		t.Errorf("the configuration was reverted %d times, want exactly 1", reverted)
	}
}
