package watch

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonProcess "github.com/luskaner/ageServer/common/process"
	"github.com/luskaner/ageServer/launcher-agent/internal"
	"github.com/luskaner/ageServer/launcher-agent/internal/gameLogs"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/cmd/agent"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-common/serverKill"
)

// What the agent says it is doing, in the tense the tray renders it in.
//
// These are the only strings here that are not also written to the log, and they
// are not a second copy of it. The log is append-only and gets read afterwards,
// by somebody who already suspects something; these are replaced as the run goes
// on and get read while it is happening, by somebody who suspects nothing.
const (
	activityWaitingForGame    = "Waiting for the game to start"
	activityWaitingForGameEnd = "Waiting for the game to end"
	activityCopyingLogs       = "Copying the game logs"
	activityStoppingServer    = "Stopping the server"
	activityStoppingBattle    = "Stopping the standalone battle server"
	activityRevertCommand     = "Running the revert command"
	activityReverting         = "Reverting the configuration"
)

var processWaitInterval = 1 * time.Second
var oneMinuteWaitTimeout = 1 * time.Minute

var (
	waitUntilAnyProcessExistFn      = waitUntilAnyProcessExist
	waitForProcessesToExitFn        = waitForProcessesToExit
	serverKillDoFn                  = serverKill.Do
	configRevertFn                  = launcherCommon.ConfigRevert
	runRevertCommandFn              = launcherCommon.RunRevertCommand
	removeBattleServerRegionFn      = launcherCommon.RemoveBattleServerRegion
	gameLogsCopyFn                  = gameLogs.CopyGameLogs
	rebroadcastFn                   = rebroadcastBattleServer
	commonProcessProcessesByNamesFn = commonProcess.ProcessesByNames
	// The buffer only decides where the wrapped work is logged, never whether it
	// runs. commonLogger.Root.Buffer is nil-safe and still calls fn(nil) on a nil
	// logger, so delegating straight to it means a run that was started without a
	// log root (agent started with no --logRoot/--baseDataPath) still performs the
	// battle-server removal, the revert command and, crucially, the configuration
	// revert. The previous "if internal.Logger == nil { return nil }" guard turned
	// "no file to log to" into "no work to do", which left the hosts file, the
	// certificate and the battle server in place after a session.
	loggerBufferFn = func(name string, fn func(io.Writer)) error {
		return internal.Logger.Buffer(name, fn)
	}
)

// ExitCode carries the agent's exit code between the goroutine that waits for
// the game to exit and the signal handler that can interrupt it, plus the
// cleanup whichever of them gets to run first.
//
// It used to be a bare *int written from both goroutines, which is a data race.
// It went unnoticed on Windows because the agent is started detached with
// CREATE_NO_WINDOW, so it has no console, Ctrl+C never reaches it and SIGTERM
// does not exist: the second writer is only reachable on the platforms where
// the signal actually fires.
type ExitCode struct {
	code atomic.Int32
}

func NewExitCode() *ExitCode {
	e := &ExitCode{}
	e.code.Store(int32(common.ErrSuccess))
	return e
}

func (e *ExitCode) Get() int {
	return int(e.code.Load())
}

// Set records code unconditionally. Reserved for the signal handler: an
// interrupt arrives out of band and has to be recorded even if something was
// already recorded.
func (e *ExitCode) Set(code int) {
	e.code.Store(int32(code))
}

// SetIfSuccess records code only while nothing has failed yet, so the first
// failure is the one that survives: a later step reporting a different error
// must not mask an earlier one. This is what every failure discovered while
// watching or cleaning up uses, including the signal handler's neighbours, so
// that an interrupt recorded first is not overwritten by the fallout it
// causes.
func (e *ExitCode) SetIfSuccess(code int) {
	e.code.CompareAndSwap(int32(common.ErrSuccess), int32(code))
}

// stopped reports whether the run was asked to stop while it waited.
//
// It is the difference between "the wait failed" and "the wait was cut short",
// which is the whole difference between a session that ended badly and one that
// ended because the user said so. Reading the context rather than being told is
// what keeps the two apart: a wait that returns early is not necessarily a stop,
// and only the caller knows whether anything asked it to.
func stopped(ctx context.Context) bool { return ctx.Err() != nil }

// session names the game and the processes being watched, for the tray's second
// row.
//
// It is built once the PIDs are known and never changes afterwards: a run that
// reports a different game half way through is a bug somewhere worse than this,
// and a row that flickers between values is worse than one that is a moment late.
func session(gameID string, names []string, pids []int) string {
	what := "no process"
	if len(pids) > 0 {
		parts := make([]string, 0, len(pids))
		for i, pid := range pids {
			if i < len(names) {
				parts = append(parts, fmt.Sprintf("%s (pid %d)", names[i], pid))
			} else {
				parts = append(parts, "pid "+strconv.Itoa(pid))
			}
		}
		what = strings.Join(parts, ", ")
	}
	if gameID == "" {
		return what
	}
	return gameID + " — " + what
}

// Cleanup is the agent's teardown: stop the server, then the battle server, the
// revert command and finally the configuration. The order matters, stopping the
// server before pulling the configuration leaves the game talking to a live
// server whose certificate trust is being removed underneath it.
//
// Each step reports only what it is doing and not which session it belongs to:
// the tray keeps the session it already knows, and there is no reason to throw
// it away halfway through the one part of the run where the user is watching.
//
// This used to exist twice with different orders (here and in cmd.runRoot's
// signal handler), so which one ran depended on how the agent was asked to stop.
// Both ways in now cancel the run instead, and the teardown runs here, once.
func Cleanup(values *agent.Values, exitCode *ExitCode) {
	if values.ServerExecutable != "" {
		internal.SetActivity(activityStoppingServer, "")
		cmdlog.Step("Killing server...")
		if err := serverKillDoFn(values.ServerExecutable); err != nil {
			cmdlog.Fail("Failed to kill server.")
			cmdlog.Fault("%s", err.Error())
			exitCode.SetIfSuccess(internal.ErrFailedStopServer)
		}
		if values.BattleServerManagerExecutable != "" && values.BattleServerRegion != "" {
			internal.SetActivity(activityStoppingBattle, "")
			cmdlog.Step("Shutting down battle-server...")
			var result *exec.Result
			logErr := loggerBufferFn("battle-server-manager_remove", func(writer io.Writer) {
				result = removeBattleServerRegionFn(
					values.BattleServerManagerExecutable, values.GameId, values.BattleServerRegion, writer, func(options *exec.Options) {
						if writer != nil {
							cmdlog.Println("run battle-server-manager", options.String())
						}
					},
				)
			})
			// Guard against a nil result before dereferencing it: the previous
			// inline version panicked here when the log buffer or the call
			// itself failed.
			if result == nil {
				result = &exec.Result{}
			}
			if logErr != nil {
				result.ExitCode = common.ErrFileLog
				result.Err = logErr
			}
			newExitCode := result.ExitCode
			if !result.Success() {
				cmdlog.Fail("Failed to shut down battle-server.")
				if result.ExitCode != common.ErrSuccess {
					cmdlog.Fault("Exit code: %d", newExitCode)
				}
				if result.Err != nil {
					cmdlog.Fault("Error: %s", result.Err)
				}
			}
			exitCode.SetIfSuccess(newExitCode)
		}
	}
	internal.SetActivity(activityRevertCommand, "")
	_ = loggerBufferFn("revert_command_end", func(writer io.Writer) {
		if err := runRevertCommandFn(writer, func(options *exec.Options) {
			cmdlog.Println("run revert command", options.String())
		}); err != nil {
			cmdlog.Fail("Failed to revert command: %s", err)
		}
	})
	internal.SetActivity(activityReverting, "")
	_ = loggerBufferFn("config_revert_end", func(writer io.Writer) {
		if !configRevertFn(values.GameId, values.LogRoot, true, writer, func(options *exec.Options) {
			if writer != nil {
				cmdlog.Println("run config revert", options.String())
			}
		}, nil) {
			cmdlog.Fail("Failed to revert configuration")
		}
	})
}

// Watch is the session: find the game, stay out of its way until it closes, and
// then put the machine back.
//
// ctx is how the run is stopped. It is cancelled by a terminate signal and by
// the Exit in the system tray, and it is what turns "wait for the game" from a
// wait nobody can end into one they can. Everything it unblocks still unwinds
// through the same deferred cleanup, so a stop and a game that closes at the same
// moment produce one teardown rather than two racing ones.
func Watch(ctx context.Context, values *agent.Values, exitCode *ExitCode, cleanupOnce *sync.Once) {
	defer func() {
		cleanupOnce.Do(func() {
			Cleanup(values, exitCode)
		})
	}()
	internal.SetActivity(activityWaitingForGame, values.GameId)
	cmdlog.Step("Waiting up to 1 minute for game to start...")
	processes := waitUntilAnyProcessExistFn(ctx, values.ProcessNames, processWaitInterval, oneMinuteWaitTimeout)
	if len(processes) == 0 {
		if stopped(ctx) {
			cmdlog.Step("Stopped before the game started.")
			return
		}
		cmdlog.Fail("Failed to find the game.")
		exitCode.SetIfSuccess(internal.ErrGameTimeoutStart)
		return
	}
	if values.BattleServerLANRebroadcast {
		port := battleServer.BroadcastPort(values.GameId)
		cmdlog.Step("Broadcasting BattleServer port to %d...", port)
		rebroadcastFn(ctx, exitCode, int(port))
	}
	names := sortedProcessNames(processes)
	var procPids []int
	var processesList []*os.Process
	for _, name := range names {
		p := processes[name]
		procPids = append(procPids, p.Pid)
		processesList = append(processesList, p)
	}
	// From here the tray has something to report beyond "starting": which game,
	// and which processes are holding the configuration in place.
	current := session(values.GameId, names, procPids)
	internal.SetActivity(activityWaitingForGameEnd, current)
	cmdlog.Step("Waiting for PIDs %v to end", procPids)
	for {
		if !waitForProcessesToExitFn(ctx, processesList) {
			if stopped(ctx) {
				// Not a failure: the user asked for this, and the exit code that
				// says so is recorded by the caller, which is the one place that can
				// tell a stop apart from a teardown that reported an error.
				cmdlog.Step("Stopped waiting for the game.")
				return
			}
			cmdlog.Fail("Failed to wait.")
			exitCode.SetIfSuccess(internal.ErrFailedWaitForProcess)
			return
		}
		// The processes found at the start are not the session. A Steam title
		// starts through a bootstrap that spawns the game and exits, and the game
		// is a new PID that carries one of the same names: ending the session at
		// the first exit reverted the configuration with the game still open,
		// which is exactly what the 2026-10-27 age3 session shows. The session
		// ends when no process with a watched name is left, not when the ones
		// seen at the start are gone.
		remaining := commonProcessProcessesByNamesFn(values.ProcessNames)
		if len(remaining) == 0 {
			break
		}
		names = sortedProcessNames(remaining)
		procPids = nil
		processesList = nil
		for _, name := range names {
			p := remaining[name]
			procPids = append(procPids, p.Pid)
			processesList = append(processesList, p)
		}
		current = session(values.GameId, names, procPids)
		internal.SetActivity(activityWaitingForGameEnd, current)
		cmdlog.Step("The game handed off to new processes, waiting for PIDs %v to end", procPids)
	}
	if values.LogRoot != "" && values.BaseDataPath != "" {
		internal.SetActivity(activityCopyingLogs, current)
		gameLogsCopyFn(values.GameId, values.BaseDataPath, values.LogRoot)
	}
}
