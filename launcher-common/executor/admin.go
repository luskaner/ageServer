package executor

import (
	"crypto/x509"
	"io"
	"net"
	"runtime"

	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/luskaner/ageServer/launcher-common/cmd/config/admin"
	"github.com/spf13/pflag"
)

// Runner executes privileged commands and reports admin status. It is injected
// into an Executor so tests can substitute a fake without mutating package
// globals (which would break t.Parallel and risk data races).
type Runner interface {
	IsAdmin() bool
	Exec(options exec.Options) *exec.Result
}

type defaultRunner struct{}

func (defaultRunner) IsAdmin() bool                    { return commonExecutor.IsAdmin() }
func (defaultRunner) Exec(o exec.Options) *exec.Result { return o.Exec() }

// Executor runs the launcher-config-admin (privileged) commands.
type Executor struct {
	runner Runner
}

// NewExecutor returns an Executor using the given Runner. A nil Runner falls
// back to the production default.
func NewExecutor(r Runner) *Executor {
	if r == nil {
		r = defaultRunner{}
	}
	return &Executor{runner: r}
}

func (e *Executor) run(flags *pflag.FlagSet, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	options := exec.Options{File: executables.NativeFileName(true, executables.LauncherConfigAdmin), AsAdmin: true, Wait: true, ExitCode: true, Args: commonCmd.FlagSetToArgs(flags, true)}
	if optionsFn != nil {
		optionsFn(&options)
	}
	if out != nil && (runtime.GOOS != "windows" || e.runner.IsAdmin() || !options.AsAdmin) {
		options.Stdout = out
		options.Stderr = out
	}
	return e.runner.Exec(options)
}

func (e *Executor) RunSetUp(gameId string, IP net.IP, macOsExclusiveMappings bool, canUseInternet bool, certificate *x509.Certificate, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	values, flags := admin.SetupFlagSet()
	values.GameId = gameId
	values.MapIp = IP
	values.MacOsExclusiveMappings = macOsExclusiveMappings
	values.CanUseInternet = canUseInternet
	values.LogRoot = logRoot
	if certificate != nil {
		values.AddLocalCertData = certificate.Raw
	}
	return e.run(flags, out, optionsFn)
}

func (e *Executor) RunRevert(IPs bool, certificate bool, failfast bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	values, flags := admin.RevertFlagSet()
	values.LogRoot = logRoot
	if failfast {
		values.IPs = IPs
		values.Certs = certificate
	} else {
		values.RemoveAll = true
	}
	return e.run(flags, out, optionsFn)
}

func (e *Executor) runFlushCache(executableName string, wait bool, IPs bool, certificate bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options), values *config.FlushCacheValues, flags *pflag.FlagSet) (file string, result *exec.Result) {
	values.IPs = IPs
	values.Certs = certificate
	values.LogRoot = logRoot
	file = executables.NativeFileName(true, executableName)
	options := exec.Options{File: file, AsAdmin: true, Args: commonCmd.FlagSetToArgs(flags, wait)}
	if wait {
		options.Wait = true
		options.ExitCode = true
	} else {
		options.Pid = true
	}
	if optionsFn != nil {
		optionsFn(&options)
	}
	if out != nil && (runtime.GOOS != "windows" || e.runner.IsAdmin() || !options.AsAdmin) {
		options.Stdout = out
		options.Stderr = out
	}
	result = e.runner.Exec(options)
	return
}

func (e *Executor) RunFlushCacheAgent(IPs bool, certificate bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (file string, result *exec.Result) {
	values, singleFs := config.FlushCacheSingleFlagSet("", nil)
	return e.runFlushCache(executables.LauncherConfigAdminAgent, false, IPs, certificate, logRoot, out, optionsFn, values, singleFs.Fs())
}

func (e *Executor) RunFlushCache(IPs bool, certificate bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (file string, result *exec.Result) {
	values, flags := config.FlushCacheFlagSet()
	return e.runFlushCache(executables.LauncherConfigAdmin, true, IPs, certificate, logRoot, out, optionsFn, values, flags)
}

// Default is the process-wide Executor used by the package-level convenience
// functions, mirroring the http.DefaultClient idiom.
var Default = NewExecutor(nil)

func RunSetUp(gameId string, IP net.IP, macOsExclusiveMappings bool, canUseInternet bool, certificate *x509.Certificate, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	return Default.RunSetUp(gameId, IP, macOsExclusiveMappings, canUseInternet, certificate, logRoot, out, optionsFn)
}

func RunRevert(IPs bool, certificate bool, failfast bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (result *exec.Result) {
	return Default.RunRevert(IPs, certificate, failfast, logRoot, out, optionsFn)
}

func RunFlushCacheAgent(IPs bool, certificate bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (file string, result *exec.Result) {
	return Default.RunFlushCacheAgent(IPs, certificate, logRoot, out, optionsFn)
}

func RunFlushCache(IPs bool, certificate bool, logRoot string, out io.Writer, optionsFn func(options *exec.Options)) (file string, result *exec.Result) {
	return Default.RunFlushCache(IPs, certificate, logRoot, out, optionsFn)
}
