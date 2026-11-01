package executor

import (
	"errors"
	"io"
	"runtime"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/certStore"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/spf13/pflag"
)

type ConfigSetupOptions struct { // Report is where the run says what it is doing. A frontend installs one
	// before starting; a nil one discards, so a caller that only wants the
	// result does not have to invent one.
	Report launcher.Reporter
	*config.SetupValues
	flags     *pflag.FlagSet
	Out       io.Writer
	OptionsFn func(options *exec.Options)
}

// report is the Reporter this was given, or one that discards. See Config's
// report for why the nil case is not an error.
func (c *ConfigSetupOptions) report() launcher.Reporter {
	if c.Report == nil {
		return launcher.Discard{}
	}
	return c.Report
}

func (c *ConfigFlushCacheOptions) report() launcher.Reporter {
	if c.Report == nil {
		return launcher.Discard{}
	}
	return c.Report
}
func NewConfigSetupOptions() *ConfigSetupOptions {
	setupValues, flags := config.SetUpFlagSet()
	return &ConfigSetupOptions{
		flags:       flags,
		SetupValues: setupValues,
	}
}

func (c *ConfigSetupOptions) ConfigRevertFlagOptions() *launcherCommon.ConfigRevertFlagOptions {
	options := launcherCommon.NewConfigRevertFlagOptions()
	options.IPs = c.MapIp != nil
	options.Certs = c.AddLocalCertData != nil
	options.RemoveUserCert = c.AddUserCertData != nil
	options.RestoreCAStoreCert = c.AddCACertData != nil
	options.GameId = c.GameId
	options.LogRoot = c.LogRoot
	options.CertFilePath = c.CertFilePath
	options.SigKeyFilePath = c.SigKeyFilePath
	options.HostFilePath = c.HostFilePath
	options.DataPath = c.DataPath
	options.GamePath = c.GamePath
	options.Metadata = c.Metadata
	options.Profiles = c.Profiles
	return options
}

func (c *ConfigSetupOptions) RunSetUp() (result *exec.Result) {
	reloadSystemCertificates := false
	reloadHostMappings := false
	c.CanUseInternet = launcher.CanUseInternet
	if logRoot := commonLogger.FileLogger.Folder(); logRoot != "" {
		c.LogRoot = logRoot
	}
	args := cmd.FlagSetToArgs(c.flags, true)
	if c.MapIp != nil {
		reloadHostMappings = true
	}
	if c.AddLocalCertData != nil || c.AddUserCertData != nil {
		reloadSystemCertificates = true
	}
	options := exec.Options{File: executables.NativeFileName(false, executables.LauncherConfig), Wait: true, Args: args, ExitCode: true}
	if c.OptionsFn != nil {
		c.OptionsFn(&options)
	}
	if c.Out != nil {
		options.Stdout = c.Out
		options.Stderr = c.Out
	}
	result = options.Exec()
	if reloadSystemCertificates {
		certStore.ReloadSystemCertificates()
	}
	if reloadHostMappings {
		common.ClearDNSCache()
	}
	if result.Success() {
		revertArgs := c.ConfigRevertFlagOptions().Flags()
		if err := launcherCommon.RevertConfigStore.Store(revertArgs); err != nil {
			c.report().Warn("Failed to store revert arguments, reverting setup...")
			result = RunRevert(revertArgs, false, c.Out, c.OptionsFn)
			if !result.Success() {
				c.report().Fail("Failed to revert setup.")
			}
			// Join both errors: the caller needs to know about the store
			// failure AND that the compensating revert may have also failed
			// (meaning the system is in an unrecoverable state).
			result.Err = errors.Join(result.Err, err)
		}
	}
	return
}

func RunRevert(flags []string, bin bool, out io.Writer, optionFn func(options *exec.Options)) (result *exec.Result) {
	values, flagSet := config.RevertFlagSet()
	if err := flagSet.Parse(flags); err != nil {
		return &exec.Result{Err: err}
	}
	result = launcherCommon.RunRevert(flags, bin, out, optionFn)
	if values.RemoveAll || values.RemoveUserCert || values.Certs {
		certStore.ReloadSystemCertificates()
	}
	if values.RemoveAll || values.IPs {
		common.ClearDNSCache()
	}
	return
}

type ConfigFlushCacheOptions struct { // Report is where the run says what it is doing. A frontend installs one
	// before starting; a nil one discards, so a caller that only wants the
	// result does not have to invent one.
	Report launcher.Reporter
	*config.FlushCacheValues
	flags *pflag.FlagSet
}

func NewConfigFlushCacheOptions(canAddHost bool, canTrustCertificate string, customHostFile bool, customCertFile bool) *ConfigFlushCacheOptions {
	ips := !customHostFile && canAddHost
	certs := !customCertFile && runtime.GOOS == "linux" && canTrustCertificate != "false"
	if !ips && !certs {
		return nil
	}
	flushCacheValues, flags := config.FlushCacheFlagSet()
	flushCacheValues.IPs = ips
	flushCacheValues.Certs = certs
	return &ConfigFlushCacheOptions{
		flags:            flags,
		FlushCacheValues: flushCacheValues,
	}
}

func (c *ConfigFlushCacheOptions) RunFlushCache() (result *exec.Result) {
	if logRoot := commonLogger.FileLogger.Folder(); logRoot != "" {
		c.LogRoot = logRoot
	}
	str := "Flushing cache"
	options := exec.Options{File: executables.NativeFileName(false, executables.LauncherConfig), Wait: true, Args: cmd.FlagSetToArgs(c.flags, true), ExitCode: true}
	if executor.IsAdmin() {
		if err := commonLogger.FileLogger.Buffer("config_flushCache", func(writer io.Writer) {
			options.Stdout = writer
			options.Stderr = writer
		}); err != nil {
			return &exec.Result{ExitCode: common.ErrFileLog}
		}
	} else {
		str += ", authorize config-admin-agent if needed"
	}
	str += "..."
	// The wait for an elevated flush is the longest step in a run and it has
	// nothing to report while it happens, so it animates in place and fills the
	// terminal's progress indicator instead of leaving one static line the reader
	// has to guess at.
	flush := commonUi.Start(str)
	flushProgress := commonUi.BeginProgress()
	commonLogger.Println("run config flushCache", options.String())
	result = options.Exec()
	if c.Certs {
		certStore.ReloadSystemCertificates()
	}
	if c.IPs {
		common.ClearDNSCache()
	}
	if !result.Success() {
		flush.Stop()
		flushProgress.Fail()
		commonLogger.Println(commonUi.Fail("Failed to flush cache"))
		if result.Err != nil {
			commonLogger.Println(commonUi.Fault("Received error:"))
			commonLogger.Println(commonUi.Detail("%s", result.Err))
		}
		if result.ExitCode != common.ErrSuccess {
			commonLogger.Println(commonUi.Fault("Received exit code:"))
			commonLogger.Println(commonUi.Detail("%d", result.ExitCode))
		}
	} else {
		flushProgress.Done()
		flush.Done("Cache flushed")
	}
	return
}
