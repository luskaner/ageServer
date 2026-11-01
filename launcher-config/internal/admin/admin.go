package admin

import (
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/logger"
	commonProcess "github.com/luskaner/ageServer/common/process"
	"github.com/luskaner/ageServer/launcher-common/executor"
	commonIpc "github.com/luskaner/ageServer/launcher-common/ipc"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config/internal"
)

// deps groups the external effect points used by the admin client so tests can
// inject fakes via newAdmin instead of mutating package globals.
type deps struct {
	bytesToCertificate func([]byte) *x509.Certificate
	newFile            func(root string, gameId string, finalRoot bool) (error, *commonLogger.Root)
	runSetUp           func(gameId string, ip net.IP, macOsExclusiveMappings bool, canUseInternet bool, certificate *x509.Certificate, logRoot string, out io.Writer, optionsFn func(*exec.Options)) *exec.Result
	runRevert          func(ips bool, certs bool, failfast bool, logRoot string, out io.Writer, optionsFn func(*exec.Options)) *exec.Result
	runFlushCache      func(ips bool, certs bool, logRoot string, out io.Writer, optionsFn func(*exec.Options)) (string, *exec.Result)
	runFlushCacheAgent func(ips bool, certs bool, logRoot string, out io.Writer, optionsFn func(*exec.Options)) (string, *exec.Result)
	process            func(name string) (string, *os.Process, error)
	killPidProc        func(pid string, proc *os.Process) error
	dialIPC            func() (net.Conn, error)
	nativeFileName     func(bin bool, name string) string
	sleep              func(time.Duration)
	getLoggerFolder    func() string
}

func defaultDeps() deps {
	return deps{
		bytesToCertificate: common.BytesToCertificate,
		newFile:            commonLogger.NewFile,
		runSetUp:           executor.RunSetUp,
		runRevert:          executor.RunRevert,
		runFlushCache:      executor.RunFlushCache,
		runFlushCacheAgent: executor.RunFlushCacheAgent,
		process:            commonProcess.Process,
		killPidProc:        commonProcess.KillPidProc,
		dialIPC:            DialIPC,
		nativeFileName:     executables.NativeFileName,
		sleep:              time.Sleep,
		getLoggerFolder: func() string {
			if internal.Logger != nil {
				return internal.Logger.Folder()
			}
			return ""
		},
	}
}

// Admin is the launcher-config-admin client. It holds both its injectable
// dependencies and its IPC connection state, so tests can construct isolated
// instances without mutating package globals.
type Admin struct {
	deps deps
	ipc  net.Conn
	enc  *gob.Encoder
	dec  *gob.Decoder
}

// Once the agent has acknowledged the shutdown all that is left on its side is
// closing the listener, flushing its log and deleting its pid file. That tail
// is short but not instant, and on a loaded machine (or with an antivirus
// scanning the log folder) it used to overrun the previous 3s budget, which is
// exactly the case that leaked an elevated process.
const (
	stopAgentPollAttempts = 100
	stopAgentPollInterval = 100 * time.Millisecond
)

// newAdmin returns an Admin using the supplied deps. Tests build their own from
// defaultDeps() plus field overrides; the production path uses Default.
func newAdmin(d deps) *Admin {
	return &Admin{deps: d}
}

// Default is the process-wide Admin used by the package-level convenience
// functions, mirroring the http.DefaultClient idiom.
var Default = newAdmin(defaultDeps())

func (a *Admin) RunSetUp(gameId string, logRoot string, ipToMap net.IP, macOsExclusiveMappings bool, addCertData []byte, canUseInternet bool) (err error, exitCode int) {
	exitCode = common.ErrGeneral
	if a.ipc != nil {
		return a.runSetUpAgent(gameId, ipToMap, macOsExclusiveMappings, canUseInternet, addCertData)
	}

	var certificate *x509.Certificate
	if addCertData != nil {
		certificate = a.deps.bytesToCertificate(addCertData)
		if certificate == nil {
			exitCode = internal.ErrUserCertAddParse
			return
		}
	}
	var result *exec.Result
	var file *commonLogger.Root
	if logRoot != "" {
		if err, file = a.deps.newFile(logRoot, "", true); err != nil {
			exitCode = common.ErrFileLog
			return
		}
	}
	var suffix string
	if len(addCertData) > 0 {
		suffix = "_cert"
	} else {
		suffix = "_hosts"
	}
	if bufferErr := file.Buffer("config-admin_setup"+suffix, func(writer io.Writer) {
		result = a.deps.runSetUp(gameId, ipToMap, macOsExclusiveMappings, canUseInternet, certificate, file.Folder(), writer, func(options *exec.Options) {
			if writer != nil {
				options.Stdout = writer
				options.Stderr = writer
			}
		})
	}); bufferErr == nil {
		err, exitCode = result.Err, result.ExitCode
	} else {
		err = bufferErr
		exitCode = common.ErrFileLog
	}
	return
}

func (a *Admin) RunRevert(logRoot string, unmapIPs bool, removeCert bool, failfast bool) (err error, exitCode int) {
	if a.ipc != nil {
		return a.runRevertAgent(unmapIPs, removeCert)
	}
	var result *exec.Result
	var file *commonLogger.Root
	if logRoot != "" {
		if err, file = a.deps.newFile(logRoot, "", true); err != nil {
			exitCode = common.ErrFileLog
			return
		}
	}
	if bufferErr := file.Buffer("config-admin_revert", func(writer io.Writer) {
		result = a.deps.runRevert(unmapIPs, removeCert, failfast, file.Folder(), writer, func(options *exec.Options) {
			if writer != nil {
				options.Stdout = writer
				options.Stderr = writer
			}
		})
	}); bufferErr == nil {
		err, exitCode = result.Err, result.ExitCode
	} else {
		err = bufferErr
		exitCode = common.ErrFileLog
	}
	return
}

// postAgentStartAttempts and postAgentStartInterval bound how long we wait for a
// freshly launched agent to become reachable.
//
// Readiness means "its IPC endpoint answers", the same predicate the caller
// checks next, and not "its pid file exists". The pid file is written before the
// agent is usable, so polling it reported a still-starting agent as ready on
// Windows the same way it reported one on Unix, which is how a slow machine got
// an agent killed for merely taking its time.
const (
	postAgentStartAttempts = 60
	postAgentStartInterval = 1 * time.Second
)

func (a *Admin) postAgentStart() bool {
	if commonExecutor.IsAdmin() {
		// Already elevated, so there is nothing to wait for.
		return true
	}
	for range postAgentStartAttempts {
		conn, err := a.deps.dialIPC()
		if err == nil {
			// Probe only: the caller connects properly right after this.
			if conn != nil {
				_ = conn.Close()
			}
			return true
		}
		a.deps.sleep(postAgentStartInterval)
	}
	return false
}

func (a *Admin) RunFlushCache(logRoot string, ips bool, certs bool) (err error, exitCode int) {
	if a.ipc != nil {
		return fmt.Errorf("cannot flush cache if agent is already started"), internal.ErrAgentAlreadyStarted
	}
	var result *exec.Result
	var file *commonLogger.Root
	if logRoot != "" {
		if err, file = a.deps.newFile(logRoot, "", true); err != nil {
			exitCode = common.ErrFileLog
			return
		}
	}
	if bufferErr := file.Buffer("config-admin_flushCache", func(writer io.Writer) {
		_, result = a.deps.runFlushCache(ips, certs, file.Folder(), writer, func(options *exec.Options) {
			if writer != nil {
				options.Stdout = writer
				options.Stderr = writer
			}
		})
	}); bufferErr == nil {
		err, exitCode = result.Err, result.ExitCode
	} else {
		err = bufferErr
		exitCode = common.ErrFileLog
	}
	return
}

func (a *Admin) StopAgentIfNeeded() bool {
	agentConnected := a.ConnectAgentIfNeeded() == nil
	exeFileName := a.deps.nativeFileName(true, executables.LauncherConfigAdminAgent)
	if !agentConnected {
		if _, proc, err := a.deps.process(exeFileName); err == nil && proc == nil {
			return true
		}
	}
	commonLogger.Println(commonUi.Step("Trying to stop config-admin-agent."))
	if err := a.stopAgentIfNeeded(); err == nil {
		for range stopAgentPollAttempts {
			if _, proc, err := a.deps.process(exeFileName); err == nil && proc == nil {
				commonLogger.Println(commonUi.Ok("Stopped config-admin-agent"))
				return true
			}
			a.deps.sleep(stopAgentPollInterval)
		}
		commonLogger.Println(commonUi.Fail("Failed to stop config-admin-agent"))
	} else {
		commonLogger.Println(commonUi.Fail("Failed to trying stopping config-admin-agent"))
		commonLogger.Println(commonUi.Detail("%s", err))
	}
	// Fallback. On Windows this only has a chance when we are already elevated:
	// the agent runs elevated, and TerminateProcess on a process at a higher
	// integrity level is denied, so from an unelevated config.exe this is not
	// merely unlikely but impossible.
	if pid, proc, err := a.deps.process(exeFileName); err == nil && proc != nil {
		if err = a.deps.killPidProc(pid, proc); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully killed config-admin-agent."))
			return true
		}
		commonLogger.Println(commonUi.Fail("Failed to kill config-admin-agent"))
		commonLogger.Println(commonUi.Detail("%s", err))
		if isAccessDenied(err) {
			// Say what actually happened and what the user can do, instead of
			// leaving a bare failure. It will not harm anything: it only waits on
			// a named pipe that nothing else is going to answer.
			commonLogger.Println(commonUi.Detail("It is running with admin privileges and this process is not, so it cannot be terminated from here."))
			commonLogger.Println(commonUi.Detail("It does nothing on its own and is harmless, and Windows will end it at sign out. To remove it now, end config-admin-agent in the task manager, or run the launcher as administrator next time."))
		}
	}
	return false
}

func (a *Admin) stopAgentIfNeeded() (err error) {
	commonLogger.Println(commonUi.Step("Stopping agent"))
	if a.ipc == nil {
		commonLogger.Println(commonUi.Info("Already stopped"))
		return
	}
	str := "-> Exit: "
	if err = a.enc.Encode(commonIpc.Exit); err != nil {
		commonLogger.Println(commonUi.Fail("%s", str+"Could not encode"))
		return
	}
	commonLogger.Println(commonUi.Ok("%s", str+"OK"))
	// Wait for the acknowledgement. The agent is elevated and this process is
	// normally not, so the IPC handshake is the only shutdown path that can
	// work here; guessing with a silent pipe is what made slow machines leak
	// the agent. A decode failure is not fatal, the caller still polls below.
	str = "<- Exit Code: "
	var exitCode int
	if decodeErr := a.dec.Decode(&exitCode); decodeErr != nil {
		commonLogger.Println(commonUi.Fail("%s", str+"Could not decode"))
	} else {
		commonLogger.Println(commonUi.Ok("%s", str+strconv.Itoa(exitCode)))
	}
	a.clearIPCState()
	return
}

// Once the agent is listening, three seconds was what it took to start it. The
// budget has to cover launching an elevated process, initialising its log and
// taking its pid lock, all of which a loaded machine can stretch. The agent
// creates its pipe before its slow work now, so this no longer has to race the
// cache flush, but it still should not be the tightest number in the code.
const (
	connectAttempts = 100
	connectInterval = 100 * time.Millisecond
)

func (a *Admin) ConnectAgentIfNeededWithRetries() bool {
	for range connectAttempts {
		if a.ConnectAgentIfNeeded() == nil {
			return true
		}
		a.deps.sleep(connectInterval)
	}
	return false
}

func (a *Admin) clearIPCState() {
	if a.ipc != nil {
		_ = a.ipc.Close()
	}
	a.enc = nil
	a.dec = nil
	a.ipc = nil
}

func (a *Admin) ConnectAgentIfNeeded() (err error) {
	commonLogger.Println(commonUi.Step("Connecting to agent"))
	if a.ipc != nil {
		commonLogger.Println(commonUi.Info("Already connected"))
		return
	}
	var conn net.Conn
	conn, err = a.deps.dialIPC()
	if err != nil {
		return
	}
	commonLogger.Println(commonUi.Ok("Connected"))
	a.ipc = conn
	a.enc = gob.NewEncoder(a.ipc)
	a.dec = gob.NewDecoder(a.ipc)
	return
}

func (a *Admin) StartAgent(flushIPs bool, flushCerts bool) (result *exec.Result) {
	commonLogger.Println(commonUi.Step("Starting agent"))
	logRoot := a.deps.getLoggerFolder()
	_, result = a.deps.runFlushCacheAgent(flushIPs, flushCerts, logRoot, nil, func(options *exec.Options) {
		commonLogger.Println(commonUi.Detail("start config-admin-agent: %s", options.String()))
	})
	if result.Success() {
		if !a.postAgentStart() {
			result.Err = fmt.Errorf("agent process failed to start")
		}
	}
	return
}

func (a *Admin) sendAgent(commandType byte, commandName string, commandFn func() any) (err error, exitCode int) {
	str := fmt.Sprintf("-> %s: ", commandName)
	if err = a.enc.Encode(commandType); err != nil {
		commonLogger.Println(commonUi.Fail("%s", str+"Could not encode"))
		return
	}
	commonLogger.Println(commonUi.Ok("%s", str+"OK"))
	str = "<- Exit Code: "
	if err = a.dec.Decode(&exitCode); err != nil || exitCode != common.ErrSuccess {
		if err != nil {
			commonLogger.Println(commonUi.Fail("%s", str+"Could not decode"))
		} else {
			commonLogger.Println(commonUi.Fail("%s", str+strconv.Itoa(exitCode)))
		}
		return
	}
	commonLogger.Println(commonUi.Ok("%s", str+strconv.Itoa(exitCode)))
	data := commandFn()
	str = fmt.Sprintf("-> %v: ", data)
	if err = a.enc.Encode(data); err != nil {
		commonLogger.Println(commonUi.Fail("%s", str+"Could not encode"))
		return
	}
	commonLogger.Println(commonUi.Ok("%s", str+"OK"))
	str = "<- Exit Code: "
	if err = a.dec.Decode(&exitCode); err != nil {
		commonLogger.Println(commonUi.Fail("%s", str+"Could not decode"))
		return
	}
	commonLogger.Println(commonUi.Ok("%s", str+strconv.Itoa(exitCode)))
	return
}

func (a *Admin) runRevertAgent(unmapIPs bool, removeCert bool) (err error, exitCode int) {
	return a.sendAgent(
		commonIpc.Revert,
		"Revert",
		func() any {
			return commonIpc.RevertCommand{IPs: unmapIPs, Certificate: removeCert}
		},
	)
}

func (a *Admin) runSetUpAgent(gameId string, mapIp net.IP, macOsExclusiveMappings bool, canUseInternet bool, certificate []byte) (err error, exitCode int) {
	return a.sendAgent(
		commonIpc.Setup,
		"Setup",
		func() any {
			return commonIpc.SetupCommand{GameId: gameId, IP: mapIp, MacOsExclusiveMappings: macOsExclusiveMappings, CanUseInternet: canUseInternet, Certificate: certificate}
		},
	)
}

// Package-level wrappers for backward compatibility. They delegate to Default.

func RunSetUp(gameId string, logRoot string, ipToMap net.IP, macOsExclusiveMappings bool, addCertData []byte, canUseInternet bool) (err error, exitCode int) {
	return Default.RunSetUp(gameId, logRoot, ipToMap, macOsExclusiveMappings, addCertData, canUseInternet)
}

func RunRevert(logRoot string, unmapIPs bool, removeCert bool, failfast bool) (err error, exitCode int) {
	return Default.RunRevert(logRoot, unmapIPs, removeCert, failfast)
}

func RunFlushCache(logRoot string, ips bool, certs bool) (err error, exitCode int) {
	return Default.RunFlushCache(logRoot, ips, certs)
}

func StopAgentIfNeeded() bool {
	return Default.StopAgentIfNeeded()
}

func ConnectAgentIfNeededWithRetries() bool {
	return Default.ConnectAgentIfNeededWithRetries()
}

func ConnectAgentIfNeeded() (err error) {
	return Default.ConnectAgentIfNeeded()
}

func StartAgent(flushIPs bool, flushCerts bool) (result *exec.Result) {
	return Default.StartAgent(flushIPs, flushCerts)
}
