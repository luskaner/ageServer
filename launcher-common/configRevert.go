package launcher_common

import (
	"io"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonProcess "github.com/luskaner/ageServer/common/process"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/spf13/pflag"
)

var RevertConfigStore = NewArgsStore(filepath.Join(os.TempDir(), common.Name+"_config_revert.txt"))

// deps groups the process-effect points used by the revert operations so tests
// can inject fakes via newReverter instead of mutating package globals (which
// would break t.Parallel and risk data races).
type deps struct {
	isAdmin      func() bool
	agentRunning func(bin bool) bool
	exec         func(options exec.Options) *exec.Result
}

func defaultDeps() deps {
	return deps{
		isAdmin:      executor.IsAdmin,
		agentRunning: ConfigAdminAgentRunning,
		exec:         func(o exec.Options) *exec.Result { return o.Exec() },
	}
}

// Reverter is the injectable entry point for the revert operations. Tests build
// their own via newReverter; the package-level functions delegate to Default.
type Reverter struct {
	deps deps
}

// newReverter returns a Reverter using the supplied deps. Any nil function is
// filled in with the production default, so callers may pass a zero deps or
// override only the fields they need.
func newReverter(d deps) *Reverter {
	def := defaultDeps()
	if d.isAdmin == nil {
		d.isAdmin = def.isAdmin
	}
	if d.agentRunning == nil {
		d.agentRunning = def.agentRunning
	}
	if d.exec == nil {
		d.exec = def.exec
	}
	return &Reverter{deps: d}
}

// Default is the process-wide Reverter used by the package-level wrappers.
var Default = newReverter(deps{})

type ConfigRevertFlagOptions struct {
	*config.RevertValues
	flags *pflag.FlagSet
}

func NewConfigRevertFlagOptions() *ConfigRevertFlagOptions {
	values, flags := config.RevertFlagSet()
	return &ConfigRevertFlagOptions{
		RevertValues: values,
		flags:        flags,
	}
}

func (c *ConfigRevertFlagOptions) Flags() []string {
	if c.RemoveAll {
		// Save originals and restore after FlagSetToArgs so callers don't see
		// surprise mutation if they reuse the options object. The FlagSet
		// values are read by FlagSetToArgs, which checks each flag's current
		// value vs default; clearing here ensures individual flags aren't
		// emitted alongside --all.
		origIPs, origCerts, origMeta, origProfiles, origUser, origCA := c.IPs, c.Certs, c.Metadata, c.Profiles, c.RemoveUserCert, c.RestoreCAStoreCert
		c.IPs = false
		c.RemoveUserCert = false
		c.Certs = false
		c.Metadata = false
		c.Profiles = false
		c.RestoreCAStoreCert = false
		defer func() {
			c.IPs = origIPs
			c.Certs = origCerts
			c.Metadata = origMeta
			c.Profiles = origProfiles
			c.RemoveUserCert = origUser
			c.RestoreCAStoreCert = origCA
		}()
	}
	return commonCmd.FlagSetToArgs(c.flags, false)
}

func allRevertFlags(gameId string, logRoot string) []string {
	options := NewConfigRevertFlagOptions()
	options.GameId = gameId
	options.LogRoot = logRoot
	options.RemoveAll = true
	return options.Flags()
}

func (r *Reverter) ConfigRevert(
	gameId string,
	logRoot string,
	headless bool,
	out io.Writer,
	optionsFn func(options *exec.Options),
	runRevertFn func(flags []string, bin bool, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result),
) (success bool) {
	if runRevertFn == nil {
		runRevertFn = r.RunRevert
	}
	err, revertFlags := RevertConfigStore.Load()
	var games []string
	if gameId == "" {
		games = game.SupportedGames.ToSlice()
	} else {
		games = []string{gameId}
	}
	multipleRevertFlags := make([][]string, len(games))
	if err != nil || len(revertFlags) > 0 {
		if err == nil {
			_, flags := config.RevertFlagSet()
			if err = flags.Parse(revertFlags); err != nil {
				commonLogger.Printf("Failed to parse revert flags: %v\n", err)
			} else {
				multipleRevertFlags = [][]string{commonCmd.FlagSetToArgs(flags, false)}
			}
		}
		if err != nil {
			if len(games) == 1 {
				commonLogger.Printf("Failed to get revert flags: %v, will revert for game %s\n", err, games[0])
			} else {
				commonLogger.Printf("Failed to get revert flags: %v, will revert for all games\n", err)
			}
			for i, g := range games {
				multipleRevertFlags[i] = allRevertFlags(g, logRoot)
			}
		}
		// This does not depend on the game type so compute it once
		requiresRevertAdminElevation := r.RevertRequiresAdminElevation(multipleRevertFlags[0], headless)
		if headless && requiresRevertAdminElevation {
			commonLogger.Println("Revert requires admin elevation while headless, this should not happen, skipping...")
			return
		}
		var revertEnd string
		if requiresRevertAdminElevation {
			revertEnd += ", authorize config-admin if needed"
		}
		for _, currentRevertFlags := range multipleRevertFlags {
			commonLogger.Println("Reverting configuration" + revertEnd + `...`)
			if revertResult := runRevertFn(currentRevertFlags, headless, out, optionsFn); revertResult.Success() {
				success = true
			} else {
				if r.deps.agentRunning(headless) {
					commonLogger.Println("\tconfig-admin-agent process is still executing. Kill it using the task manager with admin rights.")
				} else {
					commonLogger.Println("\tFailed to cleanup configuration, try to do it manually.")
				}
			}
		}
		if success {
			if err = RevertConfigStore.Delete(); err != nil {
				commonLogger.Println("Failed to clear revert flags: ", err)
			}
		}
	} else {
		success = true
	}
	return success
}

func ConfigAdminAgentRunning(bin bool) bool {
	if _, proc, err := commonProcess.Process(executables.NativeFileName(bin, executables.LauncherConfigAdminAgent)); err == nil && proc != nil {
		return true
	}
	return false
}

func (r *Reverter) RequiresAdminElevation(bin bool) bool {
	return !r.deps.isAdmin() && !r.deps.agentRunning(bin)
}

func (r *Reverter) RevertRequiresAdminElevation(args []string, bin bool) bool {
	if !r.RequiresAdminElevation(bin) {
		return false
	}
	values, flags := config.RevertFlagSet()
	// If there is an error parsing the args assume worst-case scenario, admin is needed.
	if err := flags.Parse(args); err != nil {
		commonLogger.Println("Failed to parse revert flags: ", err, ", assuming admin elevation is needed")
		return true
	}
	return RevertRequiresAdminElevationValues(values)
}

func RevertRequiresAdminElevationValues(values *config.RevertValues) bool {
	return (values.Certs && values.CertFilePath == "") ||
		(values.IPs && values.HostFilePath == "")
}

func (r *Reverter) RunRevert(flags []string, bin bool, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	args := []string{ConfigRevertCmd}
	args = append(args, flags...)
	options := exec.Options{File: executables.NativeFileName(bin, executables.LauncherConfig), Wait: true, Args: args, ExitCode: true}
	if optionsFn != nil {
		optionsFn(&options)
	}
	if out != nil {
		options.Stdout = out
		options.Stderr = out
	}
	result = r.deps.exec(options)
	return
}

// Package-level wrappers for backward compatibility. They delegate to Default.

func ConfigRevert(gameId string, logRoot string, headless bool, out io.Writer, optionsFn func(options *exec.Options), runRevertFn func(flags []string, bin bool, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result)) bool {
	return Default.ConfigRevert(gameId, logRoot, headless, out, optionsFn, runRevertFn)
}

func RequiresAdminElevation(bin bool) bool {
	return Default.RequiresAdminElevation(bin)
}

func RevertRequiresAdminElevation(args []string, bin bool) bool {
	return Default.RevertRequiresAdminElevation(args, bin)
}

func RunRevert(flags []string, bin bool, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	return Default.RunRevert(flags, bin, out, optionsFn)
}
