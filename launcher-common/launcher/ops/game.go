package ops

import (
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/executor/custom"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonProcess "github.com/luskaner/ageServer/common/process"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
	"github.com/luskaner/ageServer/launcher-common/launcher/game/battleServerBroadcast"
)

// processFn is indirected so a test can decide whether an agent is running, which
// is the difference between a teardown that prints a step and one that prints
// nothing.
var processFn = commonProcess.Process

// AgentRunning reports whether there is a config-admin-agent process to stop.
func (c *Config) AgentRunning() bool {
	_, proc, err := processFn(executables.NativeFileName(false, executables.LauncherAgent))
	return err == nil && proc != nil
}

// KillAgent stops the agent, and says so only when there was one.
//
// Announcing "Stopping config-admin-agent..." over an agent that is not running
// would be a step that did not happen, and on a clean run that is every run.
func (c *Config) KillAgent() {
	agent := executables.NativeFileName(false, executables.LauncherAgent)
	if _, proc, err := processFn(agent); err != nil || proc == nil {
		return
	}
	c.report().Step("Stopping config-admin-agent...")
	if err := commonProcess.Kill(agent); err != nil {
		c.report().Warn("Failed to kill it: %s, try using the task manager.", err)
		return
	}
	c.report().Ok("config-admin-agent stopped.")
}

// clientArgsValues are the replacement values the client launcher arguments may
// use: the files this run produced for it to read.
//
// On Windows the separators are doubled because these arguments are split the
// same way as a command line, where a lone backslash is an escape character.
// Nothing else about the paths is rewritten: the client is told where the file
// actually is.
func (c *Config) clientArgsValues() map[string]string {
	values := make(map[string]string)
	add := func(name string, path string) {
		if path == "" {
			return
		}
		if runtime.GOOS == "windows" {
			path = strings.ReplaceAll(path, `\`, `\\`)
		}
		values[name] = path
	}
	add("HostFilePath", c.hostFilePath)
	add("CertFilePath", c.certFilePath)
	// Converted before add escapes it: the client's signing key DLL reads this
	// one from a command line it splits on spaces, so it may need the short form
	// of the path, whose separators must not then be doubled twice.
	sigKeyPath, _ := sigKeyArgPath(c.sigKeyFilePath)
	add("SigKeyFilePath", sigKeyPath)
	if len(values) == 0 {
		return nil
	}
	return values
}

func (c *Config) LaunchAgentAndGame(executer base.Executor, customExecutor custom.Exec, clientExecutableArgs []string, canTrustCertificate string, canBroadcastBattleServer string, basePath string) (exitCode int) {
	if canBroadcastBattleServer != "false" {
		if battleServerBroadcast.Required() {
			canBroadcastBattleServer = "true"
		} else {
			canBroadcastBattleServer = "false"
		}
	}
	loggerPath := commonLogger.FileLogger.Folder()
	revertCommand := c.RevertCommand()
	requiresConfigRevert := c.RequiresConfigRevert()
	if loggerPath != "" || len(revertCommand) > 0 || canBroadcastBattleServer == "true" || len(c.serverExe) > 0 || requiresConfigRevert {
		str := "Starting agent"
		if canBroadcastBattleServer == "true" {
			str += ", authorize it in firewall if needed"
		}
		c.report().Step("%s", str+"...")
		steamProcess, steamMacOsNative, xboxProcess := executer.GameProcesses()
		var err error
		var f *os.File
		if f, err = commonLogger.FileLogger.Open("agent"); err != nil {
			c.report().Fail("Error message: %s", err.Error())
			return common.ErrFileLog
		}
		// Convert explicitly: assigning a nil *os.File directly to io.Writer
		// produces a non-nil interface holding a nil pointer, which passes
		// StartAgent's `out != nil` guard and breaks the child process.
		var out io.Writer
		if f != nil {
			out = f
		}
		result := executor.StartAgent(
			c.gameId,
			steamProcess,
			steamMacOsNative,
			xboxProcess,
			c.serverExe,
			canBroadcastBattleServer == "true",
			c.battleServerExe,
			c.battleServerRegion,
			basePath,
			loggerPath,
			out,
			func(options *commonExecutor.Options) {
				commonLogger.Println("start agent", options.String())
			},
		)
		// Close the parent's handle: the child inherited its own copy.
		_ = f.Close()
		if !result.Success() {
			c.report().Fail("Failed to start agent.")
			exitCode = launcher.ErrAgentStart
			if result.Err != nil {
				c.report().Fault("Error message: %s", result.Err.Error())
			}
			if result.ExitCode != common.ErrSuccess {
				c.report().Fault("Exit code: %d.", result.ExitCode)
			}
			return
		}

		c.report().Ok("Agent started.")
	}
	str := "Starting game"
	if customExecutor.Executable != "" {
		str += ", authorize it if needed"
	}
	c.report().Step("%s", str+"...")
	var result *commonExecutor.Result
	values := c.clientArgsValues()
	args, err := ParseCommandArgs(clientExecutableArgs, values)
	if err != nil {
		c.report().Fail("Failed to parse client executable arguments")
		exitCode = launcher.ErrInvalidClientArgs
		return
	}

	if result = executer.Do(args, func(options commonExecutor.Options) {
		commonLogger.Println("start game", options.String())
	}); !result.Success() && result.Err != nil {
		if customExecutor.Executable != "" && adminError(result) {
			if canTrustCertificate == "user" {
				c.report().Warn("Using a user certificate. If it fails to connect to the server, try setting the config setting Config.Certificate.CanTrustInPc to \"local\".")
			}
			result = customExecutor.DoElevated(args, func(options commonExecutor.Options) {
				commonLogger.Println("start elevated game", options.String())
			})
		}
	}
	if !result.Success() {
		exitCode = launcher.ErrGameLauncherStart
		if result.Err != nil {
			c.report().Fail("Game failed to start. Error message: %s", result.Err.Error())
		}
		c.KillAgent()
	} else {
		c.report().Ok("Game started.")
	}
	return
}
