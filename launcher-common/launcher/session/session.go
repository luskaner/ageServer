package session

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/luskaner/ageServer/common/uuid"

	"slices"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmd/bsManager"
	cmdServer "github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/fileLock"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/cert"
	gameExecutor "github.com/luskaner/ageServer/common/game/executor"
	"github.com/luskaner/ageServer/common/game/executor/custom"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonProcess "github.com/luskaner/ageServer/common/process"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
	"github.com/luskaner/ageServer/launcher-common/launcher/ops"
	"github.com/luskaner/ageServer/launcher-common/launcher/ops/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher/server"
	"github.com/spf13/pflag"
)

// Package session performs a launcher session: the whole thing a launch does,
// from reading its configuration to putting the machine back.
//
// It is separate from launcher-common/launcher because the operations it drives
// depend on the vocabulary that package defines, and Go does not allow a package
// to import something that already imports it. The vocabulary stays where it is
// and the session sits next to it.

// BindFlags registers every launcher option on fs, writing into the storage this
// package reads them from.
//
// The options live here rather than in a frontend because a session reads them
// and nothing else does. A frontend that wants its own storage instead of
// calling this registers the same options with launcher.BindFlags on a flag set
// of its own, and names that storage in Setup.Values, which is where the run then
// reads them from.
func BindFlags(fs *pflag.FlagSet) error {
	return launcher.BindFlags(fs, launcher.Values{
		ConfigFile:     &cfgFile,
		GameConfigFile: &gameCfgFile,
		GameID:         &gameId,
		Output:         &output,
	})
}

// Setup is what a frontend hands over before performing a session.
//
// Everything in it is a question the session cannot answer for itself: what to
// call itself in the summary, where to say what it is doing, and how to put a
// question to someone. None of those differ between two frontends doing the same
// work, which is why they are asked for once rather than guessed at.
type Setup struct {
	// Version is shown in the session summary.
	Version string
	// Values is where the run reads the options it was given.
	//
	// A frontend that registered them with BindFlags leaves it nil, and the run
	// reads the storage that call wrote into. A frontend that keeps its options
	// somewhere else installs that storage here instead: a window, whose options
	// are the values its controls hold rather than words on a command line.
	//
	// It is the same launcher.Values that launcher.LoadConfig takes, which is the
	// point of it. A frontend can resolve the configuration before performing a
	// run, show what the run would use, and hand the very same storage over to
	// perform it, rather than asking twice and hoping the two answers match.
	Values *launcher.Values
	// Report is where the session narrates itself. A console writes it to the
	// terminal and the log file; a window writes it to an event stream. There is
	// no default beyond discarding, so a frontend that forgets gets silence
	// rather than lines on a terminal it does not own.
	Report launcher.Reporter
	// NewDialog resolves the configured dialog mode to the backend that asks
	// the questions, and says why it fell back when the configured choice
	// could not be honoured.
	//
	// The session installs whatever it returns with launcher.SetDialog before
	// asking anything, so a test cannot inject a fake backend through the
	// registry alone: it has to own the whole resolver.
	NewDialog func(enable bool) launcher.Resolution
	// Stdin is what a console backend reads its answers from. A frontend that
	// asks in its own window leaves it nil and its backend ignores it.
	Stdin io.Reader
	// Presenter is how the session shows where it is: the banner, the headings,
	// the summary and the indicators that run while something is in progress.
	//
	// It lands in a slot rather than being passed down, because the operations
	// that draw are several layers below the session and are shared. A frontend
	// that leaves it nil gets a run with no decoration.
	Presenter launcher.Presenter
}

// Configure installs what a frontend brings to the session. It is safe to call
// more than once, and a later call replaces an earlier one: a test that installs
// its own reporter must not have to undo someone else's.
func Configure(s Setup) {
	setup = s
	launcher.SetPresenter(s.Presenter)
}

// Current returns what a frontend installed. It exists so that a frontend's own
// tests can check what they handed over, and so that an embedder can see the
// defaults rather than having to remember them.
func Current() Setup { return setup }

var setup Setup

// optionStorage returns where the current run reads its options from: the
// frontend's own when it installed one, and the storage BindFlags wrote into
// otherwise.
//
// It is a function rather than a field because the answer changes when a frontend
// calls Configure, and a run resolves it once on its way in rather than reading
// Setup from four layers down.
func optionStorage() launcher.Values {
	if setup.Values != nil {
		return *setup.Values
	}
	return launcher.Values{
		ConfigFile:     &cfgFile,
		GameConfigFile: &gameCfgFile,
		GameID:         &gameId,
		Output:         &output,
	}
}

// option reads one option from wherever this run's options live, treating an
// absent target as the empty value rather than as a nil to dereference: a
// frontend that installed storage of its own may have left one of them out, and a
// run must report what it was given rather than panic reading it.
func option(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// dialogNewFn resolves the configured dialog mode. It is a variable rather than
// a direct call so that a test can own the whole backend: the session installs
// whatever this returns, so a fake injected only through launcher.SetDialog would
// be replaced before the first question.
var dialogNewFn = func(enable bool) launcher.Resolution {
	if setup.NewDialog == nil {
		return launcher.Resolution{}
	}
	return setup.NewDialog(enable)
}

// report is the Reporter for the session's own lines, which is the frontend's
// when it installed one.
func reportOrDiscard() launcher.Reporter {
	if setup.Report != nil {
		return setup.Report
	}
	return launcher.Discard{}
}

// promptInput is where a console backend reads its answers from, defaulting to
// the process's own standard input.
//
// It is read once per run rather than passed down: the prompts happen several
// layers below the session, inside operations that are shared, and threading a
// reader through all of them to say "the terminal" would be noise. A frontend
// that asks in a window installs a backend that reads nothing, and never gets
// here.
func promptInput() io.Reader {
	if setup.Stdin != nil {
		return setup.Stdin
	}
	return os.Stdin
}

var config = &ops.Config{}

// report is how the shared logic talks to this frontend.

var (
	Version      string
	cfgFile      string
	gameCfgFile  string
	gameId       string
	output       string
	filesToPrint []string
	// usedConfigFile is the main config file initConfig ended up loading, kept
	// next to filesToPrint so the session summary can name it.
	usedConfigFile string
)

var (
	initConfigFn      = initConfig
	newPidLockFn      = func() fileLock.Locker { return &fileLock.PidLock{} }
	isAdminFn         = func() bool { return commonExecutor.IsAdmin() }
	openMainLogFn     = logger.OpenMainFileLog
	printFileFn       = logger.PrintFile
	writeFileLogFn    = logger.WriteFileLog
	dnsConnectivityFn = common.DNSConnectivity
)

var (
	gameSupportedGamesContainsOneFn = func(gameId string) bool { return game.SupportedGames.ContainsOne(gameId) }
	parseCommandArgsFn              = ops.ParseCommandArgs
	resolveIsolateValueFn           = ops.ResolveIsolateValue
	configSetGameIdFn               = config.SetGameId
	configIsolationPathFn           = config.IsolationPath
	configGamePathToGameCertPathFn  = config.GamePathToGameCertPath
	configNativeMacOsGameFn         = config.NativeMacOsGame
	configBattleServerRequiredFn    = config.BattleServerRequired
	makeExecFn                      = gameExecutor.MakeExec
	commonParsePathFn               = common.ParsePath
	commonEnhancedViperFn           = common.EnhancedViperStringToStringSlice
	executablesFindPathFn           = executables.FindPath
	commonProcessProcessFn          = commonProcess.Process
	commonProcessWaitForProcessFn   = commonProcess.WaitForProcessContext
	gameRunningFn                   = func() bool { return ops.GameRunning(reportOrDiscard()) }
	configKillAgentFn               = config.KillAgent
	launcherCommonConfigRevertFn    = launcherCommon.ConfigRevert
	executorRunRevertFn             = executor.RunRevert
	commonLoggerFileLoggerBufferFn  = func(name string, fn func(io.Writer)) error {
		if commonLogger.FileLogger == nil {
			return nil
		}
		return commonLogger.FileLogger.Buffer(name, fn)
	}
	newConfigFlushCacheOptionsFn = executor.NewConfigFlushCacheOptions
	executablesNativeFileNameFn  = executables.NativeFileName
	configRunSetupCommandFn      = config.RunSetupCommand
	netipParseAddrFn             = netip.ParseAddr
	discoverServersFn            = func(gameTitle string, singleAutoSelect bool, multicastGroups mapset.Set[netip.Addr], targetPorts mapset.Set[uint16]) (uuid.UUID, net.IP) {
		return ops.DiscoverServersAndSelectBestIpAddr(reportOrDiscard(), promptInput(), gameTitle, singleAutoSelect, multicastGroups, targetPorts)
	}
	serverGetExecutablePathFn = server.GetExecutablePath
	serverGenerateCertsFn     = func(serverExecutablePath string, canTrustCertificate bool) int {
		return server.GenerateServerCertificates(reportOrDiscard(), serverExecutablePath, canTrustCertificate)
	}
	configRunBattleServerManagerFn  = config.RunBattleServerManager
	configStartServerFn             = config.StartServer
	serverReadCACertFn              = server.ReadCACertificateFromServer
	configMapHostsFn                = config.MapHosts
	configAddCertFn                 = config.AddCert
	configIsolateUserDataFn         = config.IsolateUserData
	configAddCACertToGameFn         = config.AddCACertToGame
	configLaunchAgentAndGameFn      = config.LaunchAgentAndGame
	configRunStopAgentFn            = config.RunStopAgent
	serverFilterServerIPsFn         = server.FilterServerIPs
	bsManagerStartFlagSetFn         = bsManager.StartFlagSet
	commonHostOrIpToIpsFn           = common.HostOrIpToIps
	commonStringSliceToNetIPSliceFn = common.StringSliceToNetIPSlice
	commonNetIPSliceToNetIPSetFn    = common.NetIPSliceToNetIPSet
	uuidParseFn                     = uuid.Parse
	uuidMustParseFn                 = uuid.MustParse
	uuidNilFn                       = uuid.Nil
	signalNotifyFn                  = signal.Notify
	signalStopFn                    = signal.Stop
)

func Run(ctx context.Context, fs *pflag.FlagSet) (err error, exitCode int) {
	// Resolved once, here, and read through for the rest of the run: the options
	// do not move, but which storage they are in depends on the frontend, and
	// reading the same place every time is what keeps the two from disagreeing.
	options := optionStorage()
	gameId := option(options.GameID)
	// validate required flags
	if gameId == "" {
		return errors.New("required flag 'game' not set"), common.ErrSyntax
	}
	// The explicit flag gets the last word over AGE_LANSERVER_OUTPUT, and this is
	// the first point where the parsed flags are available.
	launcher.ActivePresenter().ApplyOutput(option(options.Output))

	lock := newPidLockFn()
	if err = lock.Lock(); err != nil {
		reportOrDiscard().Fail("Failed to lock pid file. Kill process launcher if it is running in your task manager.")
		reportOrDiscard().Detail("%s", err.Error())
		exitCode = common.ErrPidLock
		return
	}
	cfg := initConfigFn(fs)
	// Everything the run does from here reports through this, so the same session
	// could be read in a terminal or in a window by installing a different one.
	config.Report = reportOrDiscard()
	// Whatever a previous session in this process changed is not this run's to
	// undo, and reporting that it changed something it did not would send the
	// teardown after a server this run never started.
	config.Reset()
	logger.LogEnabled = cfg.Config.Log
	if err = openMainLogFn(gameId); err != nil {
		reportOrDiscard().Fail("Failed to open file log")
		reportOrDiscard().Detail("%s", err.Error())
		exitCode = common.ErrFileLog
		return
	}
	if !cfg.Config.CanUseInternet {
		launcher.CanUseInternet = false
		reportOrDiscard().Info("Internet usage is disabled via config.")
	} else {
		launcher.CanUseInternet = dnsConnectivityFn()
	}
	if !launcher.CanUseInternet {
		reportOrDiscard().Warn("No internet connectivity, some features will fallback gracefully.")
	}
	common.SetUseInternet(launcher.CanUseInternet)
	for _, fileToPrint := range filesToPrint {
		printFileFn("config", fileToPrint)
	}
	var atomicExitCode atomic.Int32
	atomicExitCode.Store(int32(common.ErrSuccess))
	// Teardown runs at most once and never concurrently. The signal handler
	// below and this deferred cleanup could both decide to tear down; unguarded
	// they raced, spawning two config.exe reverts over one hosts lock and two
	// stop-agent clients on one named pipe, which is one way an elevated
	// config-admin-agent outlived the launcher. The mutex also makes the
	// second caller wait instead of returning, so the signal handler's
	// os.Exit cannot truncate a revert that is still in flight.
	var teardownMutex sync.Mutex
	teardownRan := false
	teardown := func(force bool) {
		teardownMutex.Lock()
		defer teardownMutex.Unlock()
		if teardownRan {
			return
		}
		teardownRan = true
		// A taskbar button left half filled after the program is gone is worse than
		// no button at all, and this is the one path every exit goes through.
		launcher.ActivePresenter().ClearProgress()
		// force comes from the signal handler, where the user asked to stop
		// and the exit code says nothing about how far setup got.
		//
		// The heading is printed only when there is something to undo. On a clean run
		// each step reverted its own changes as it finished, so all that is left here
		// is closing a log and releasing a lock: a heading over that announces work
		// that is not happening, which is worse than no heading at all.
		if force || atomicExitCode.Load() != int32(common.ErrSuccess) {
			if config.HasTeardownWork() {
				launcher.ActivePresenter().Section("Final teardown")
			}
			config.Revert()
		}
		logger.WriteFileLog(gameId, "before exit")
		commonLogger.CloseFileLog()
		_ = lock.Unlock()
	}
	defer func() {
		if r := recover(); r != nil {
			reportOrDiscard().Fail("%s", r)
			reportOrDiscard().Detail("%s", string(debug.Stack()))
			atomicExitCode.Store(int32(common.ErrGeneral))
		}
		teardown(false)
		exitCode = int(atomicExitCode.Load())
	}()
	dialogResolution := dialogNewFn(cfg.Config.CanUseDialogs)
	launcher.SetDialog(dialogResolution.Dialog)
	// Registered after the teardown defer so, by LIFO, it runs before it.
	defer launcher.ResetDialog()
	// The console's presenter draws nothing when the console width is unknown, so a
	// redirected run or a pipe never gets a banner in the middle of its output.
	launcher.ActivePresenter().Banner(executables.Launcher, setup.Version)
	// One place for the three facts the run depends on. They used to be announced
	// once here and once again further down, which is how a mismatch between the
	// two copies of the same fact looked like a bug.
	launcher.ActivePresenter().Section("Configuration")
	launcher.ActivePresenter().KV(0, "main config file", orNone(usedConfigFile))
	launcher.ActivePresenter().KV(0, "game config file", orNone(option(options.GameConfigFile)))
	launcher.ActivePresenter().KV(0, "game", gameId)
	// Printed only when the console could show more than it is showing, because
	// the fallback is a code page and not a decision: without this line the ASCII
	// markers look like the intended output rather than like what they are.
	if hint := launcher.ActivePresenter().Hint(); hint != "" {
		reportOrDiscard().Info("%s", hint)
	}
	// The backend in use is not worth a line of its own: a graphical dialog is
	// visible, and a console one answers on the terminal. Only the fallback is
	// news, because it means the configured choice was not honoured.
	if dialogResolution.Reason != "" {
		reportOrDiscard().Warn("%s", dialogResolution.Reason)
	}
	writeFileLogFn(gameId, "start")
	isAdmin := isAdminFn()
	canTrustCertificate := cfg.Config.Certificate.CanTrustInPc
	if ec := launcher.ValidateCanTrustCertificate(reportOrDiscard(), canTrustCertificate); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	canBroadcastBattleServer := "false"
	if runtime.GOOS == "windows" && (gameId != game.AoM && gameId != game.AoE4) {
		canBroadcastBattleServer = cfg.Config.CanBroadcastBattleServer
		if ec := launcher.ValidateCanBroadcastBattleServer(reportOrDiscard(), canBroadcastBattleServer); ec != common.ErrSuccess {
			atomicExitCode.Store(int32(ec))
			return
		}
	}
	serverStart := cfg.Server.Start
	if ec := launcher.ValidateServerStartValue(reportOrDiscard(), serverStart); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	serverStop := cfg.Server.Stop
	if ec := launcher.ValidateServerStopValue(reportOrDiscard(), serverStop, runtime.GOOS != "windows" && isAdmin); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	battleServerManagerRun := cfg.Server.BattleServerManager.Run
	if ec := launcher.ValidateRequiredTrueFalse(reportOrDiscard(), battleServerManagerRun, "Server.BattleServerManager.Run", launcher.RequiredTrueFalseValues()); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	isolateMetadataStr := cfg.Client.Isolation.Metadata
	if ec := launcher.ValidateRequiredTrueFalse(reportOrDiscard(), isolateMetadataStr, "Client.Isolation.Metadata", launcher.RequiredTrueFalseValues()); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	isolateProfilesStr := cfg.Client.Isolation.Profiles
	if ec := launcher.ValidateRequiredTrueFalse(reportOrDiscard(), isolateProfilesStr, "Client.Isolation.Profiles", launcher.RequiredTrueFalseValues()); ec != common.ErrSuccess {
		atomicExitCode.Store(int32(ec))
		return
	}
	if !gameSupportedGamesContainsOneFn(gameId) {
		reportOrDiscard().Fail("Invalid game type")
		atomicExitCode.Store(int32(launcherCommon.ErrInvalidGame))
		return
	}
	configSetGameIdFn(gameId)
	serverValues := map[string]string{
		"Game":             gameId,
		"Id":               uuid.New().String(),
		"StartBsIfMissing": "true",
	}
	var serverArgsValues *cmdServer.Values
	var serverFlags *pflag.FlagSet
	var serverArgs []string
	if serverArgs, err = parseCommandArgsFn(cfg.Server.Args, serverValues); err == nil {
		var serverSingleFlagSet *commonCmd.SingleFlagSet
		serverArgsValues, serverSingleFlagSet = cmdServer.SingleFlagSet("", nil, nil)
		serverFlags = serverSingleFlagSet.Fs()
		if err = serverFlags.Parse(serverArgs); err != nil {
			reportOrDiscard().Fail("Failed to parse server executable arguments")
			atomicExitCode.Store(int32(launcher.ErrInvalidServerArgs))
			return
		}
		if _, err = uuidParseFn(serverArgsValues.Id); err != nil {
			reportOrDiscard().Fail("You must provide a valid UUID for the server ID using the --id argument in server executable arguments")
			atomicExitCode.Store(int32(launcher.ErrInvalidServerArgs))
			return
		}
	} else {
		reportOrDiscard().Fail("Failed to parse server executable arguments")
		atomicExitCode.Store(int32(launcher.ErrInvalidServerArgs))
		return
	}
	var battleServerManagerArgs []string
	battleServerManagerArgs, err = parseCommandArgsFn(
		cfg.Server.BattleServerManager.Args,
		serverValues,
	)
	if err != nil {
		reportOrDiscard().Fail("Failed to parse battle-server-manager executable arguments")
		atomicExitCode.Store(int32(launcher.ErrInvalidServerBattleServerManagerArgs))
		return
	}
	var setupCommand []string
	setupCommand, err = parseCommandArgsFn(cfg.Config.SetupCommand, nil)
	if err != nil {
		reportOrDiscard().Fail("Failed to parse setup command")
		atomicExitCode.Store(int32(launcher.ErrInvalidSetupCommand))
		return
	}
	var revertCommand []string
	revertCommand, err = parseCommandArgsFn(cfg.Config.RevertCommand, nil)
	if err != nil {
		reportOrDiscard().Fail("Failed to parse revert command")
		atomicExitCode.Store(int32(launcher.ErrInvalidRevertCommand))
		return
	}
	canAddHost := cfg.Config.CanAddHost
	clientExecutable := cfg.Client.Executable.Path
	if clientExecutable == "steam" && runtime.GOOS == "darwin" && gameId != game.AoE2 {
		reportOrDiscard().Fail("Only AoE 2: DE is supported on 'steam'. Use 'steam_crossover' or 'steam_wine' instead.")
		atomicExitCode.Store(int32(launcher.ErrGameUnsupportedLauncherCombo))
		return
	}
	var clientExecutableOfficial bool
	if clientExecutable == "auto" || clientExecutable == "steam" {
		clientExecutableOfficial = true
	} else if runtime.GOOS == "windows" {
		clientExecutableOfficial = clientExecutable == "msstore"
	} else if clientExecutable == "steam_wine" || clientExecutable == "steam_crossover" {
		clientExecutableOfficial = true
	}
	var isolateMetadata bool
	if gameId != game.AoE1 {
		isolateMetadata = resolveIsolateValueFn(isolateMetadataStr, clientExecutableOfficial)
	}
	isolateProfiles := resolveIsolateValueFn(isolateProfilesStr, clientExecutableOfficial)
	isolation := isolateMetadata || isolateProfiles
	var isolationPath string
	if isolation {
		if cfg.Client.Isolation.Path != "auto" {
			var isolationDir os.FileInfo
			if isolationDir, isolationPath, err = commonParsePathFn(commonEnhancedViperFn(cfg.Client.Isolation.Path), nil); err != nil || !isolationDir.IsDir() {
				reportOrDiscard().Fail("Invalid isolation path")
				atomicExitCode.Store(int32(launcher.ErrInvalidIsolationPath))
				return
			}
			logger.SetBasePath(isolationPath)
			logger.WriteFileLog(gameId, "post isolation path")
		} else if runtime.GOOS != "windows" && !clientExecutableOfficial {
			reportOrDiscard().Fail("You must set the Client.Isolation.Path as you are using a custom launcher with isolation.")
			atomicExitCode.Store(int32(launcher.ErrInvalidIsolationPath))
			return
		}
	}
	var serverExecutable string
	if serverExecutable = cfg.Server.Executable.Path; serverExecutable != "auto" {
		var serverFile os.FileInfo
		if serverFile, serverExecutable, err = commonParsePathFn(commonEnhancedViperFn(cfg.Server.Executable.Path), nil); err != nil || serverFile.IsDir() {
			reportOrDiscard().Fail("Invalid server executable")
			atomicExitCode.Store(int32(launcher.ErrInvalidServerPath))
			return
		}
	}
	var battleServerManagerExecutable string
	if battleServerManagerExecutable = cfg.Server.BattleServerManager.Executable.Path; battleServerManagerExecutable != "auto" {
		var battleServerManagerFile os.FileInfo
		if battleServerManagerFile, battleServerManagerExecutable, err = commonParsePathFn(commonEnhancedViperFn(cfg.Server.BattleServerManager.Executable.Path), nil); err != nil || battleServerManagerFile.IsDir() {
			reportOrDiscard().Fail("Invalid battle-server-manager executable")
			atomicExitCode.Store(int32(launcher.ErrInvalidClientPath))
			return
		}
	}
	if !clientExecutableOfficial {
		var clientFile os.FileInfo
		if clientFile, clientExecutable, err = commonParsePathFn(commonEnhancedViperFn(cfg.Client.Executable.Path), nil); err != nil || clientFile.IsDir() {
			reportOrDiscard().Fail("Invalid client executable")
			atomicExitCode.Store(int32(launcher.ErrInvalidClientPath))
			return
		}
	} else if !isolateProfiles || (gameId != game.AoE1 && !isolateMetadata) {
		reportOrDiscard().Fail("Isolating profiles and metadata is a must when using an official launcher.")
		atomicExitCode.Store(int32(launcher.ErrRequiredIsolation))
		return
	} else {
		reportOrDiscard().Warn("Make sure you disable the cloud saves in the launcher settings to avoid issues.")
	}

	if isAdmin {
		reportOrDiscard().Warn("Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when needed.")
		if runtime.GOOS != "windows" {
			reportOrDiscard().Detail("It can also cause issues and restrict the functionality.")
		}
	}

	serverHost := cfg.Server.Host

	if clientExecutable == "msstore" && gameId == game.AoM {
		reportOrDiscard().Fail("The Microsoft Store (Xbox) version is not supported on this game.")
		atomicExitCode.Store(int32(launcher.ErrGameUnsupportedLauncherCombo))
		return
	}
	configSetGameIdFn(gameId)
	// Finding the game is the first thing that takes long enough to look broken,
	// so it gets an in place line and the terminal's own progress indicator rather
	// than a "Step" the reader has to trust is still running.
	gameSearch := launcher.ActivePresenter().Start("Looking for the game...")
	gameProgress := launcher.ActivePresenter().BeginProgress()
	var gamePath string
	executer := makeExecFn(gameId, clientExecutable)
	if executer != nil {
		gameProgress.Done()
		gameSearch.Done("Game found on %s.", executer.String())
	} else {
		gameProgress.Fail()
		gameSearch.Fail("Game not found.")
		atomicExitCode.Store(int32(launcher.ErrGameLauncherNotFound))
		return
	}
	if isolation && isolationPath == "" {
		if isolationPath = configIsolationPathFn(executer); isolationPath == "" {
			reportOrDiscard().Fail("Failed to auto retrieve isolation path")
			atomicExitCode.Store(int32(launcher.ErrInvalidIsolationPath))
			return
		}
		logger.SetBasePath(isolationPath)
		logger.WriteFileLog(gameId, "post isolation path")
	}
	var customExecutor custom.Exec
	var ok bool
	if customExecutor, ok = executer.(custom.Exec); ok {
		if cert.HasCA(gameId) {
			var clientFile os.FileInfo
			var clientPath string
			if clientFile, clientPath, err = commonParsePathFn(commonEnhancedViperFn(cfg.Client.Path), nil); err != nil || !clientFile.IsDir() {
				reportOrDiscard().Fail("Invalid client path")
				atomicExitCode.Store(int32(launcher.ErrInvalidClientPath))
				return
			}
			gamePath = clientPath
		}
	} else if cert.HasCA(gameId) {
		gamePath = executer.(game.Locatable).Path()
	}
	var gameCaCertPath string
	if gamePath != "" {
		_, caCert := cert.NewCA(gameId, configGamePathToGameCertPathFn(executer, gamePath))
		gameCaCertPath = caCert.OriginalPath()
		if commonLogger.FileLogger != nil {
			logger.SetCacert(&caCert)
		}
	}
	macOsExclusiveMappings := configNativeMacOsGameFn(executer, true)
	if commonLogger.FileLogger != nil {
		logger.SetMacOsExclusiveMappings(macOsExclusiveMappings)
	}
	// Read once, here, rather than where it is used. This handler outlives the
	// call that started it, and a frontend that starts another run in between
	// would otherwise have this one stop the signals of the other.
	stopSignals := signalStopFn
	sigs := make(chan os.Signal, 1)
	signalNotifyFn(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals(sigs)
	// The signal asks for a stop; it does not perform one. Forcing the teardown
	// here is deliberate: whoever sent it did not wait for the run to reach a
	// convenient point, and a revert that had not started yet still has to
	// happen. Cancelling rather than exiting lets the run unwind on its own,
	// which is what keeps a revert in flight from being cut off.
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() {
		select {
		case _, sigOk := <-sigs:
			if sigOk {
				teardown(true)
				cancelRun()
			}
		case <-ctx.Done():
		}
	}()
	// cancelled reports whether the run was asked to stop before this point, and
	// says so once instead of at every check.
	var stopped bool
	cancelled := func(at string) bool {
		if stopped || ctx.Err() == nil {
			return stopped
		}
		stopped = true
		reportOrDiscard().Step("Stopped before %s.", at)
		atomicExitCode.Store(int32(launcher.ErrCanceled))
		return true
	}
	agentWaitDuration := time.Minute
	agent := executablesNativeFileNameFn(false, executables.LauncherAgent)
	if _, proc, localErr := commonProcessProcessFn(agent); localErr == nil && proc != nil {
		reportOrDiscard().Step("agent is running, waiting up to %s for it to end...", agentWaitDuration)
		if !commonProcessWaitForProcessFn(ctx, proc, &agentWaitDuration) {
			reportOrDiscard().Warn("agent did not exit on its own.")
		}
	}
	if _, proc, localErr := commonProcessProcessFn(executablesNativeFileNameFn(false, executables.LauncherConfigAdminAgent)); localErr == nil && proc != nil {
		// It never exits on its own: only an explicit 'Exit' sent over the IPC
		// pipe stops it. This used to just wait for it, which stalled every
		// launch for the full timeout and then left the elevated process in
		// place, still holding the mapped-ips/cert state of the previous
		// session so the next setUp failed with 'already mapped'. Ask it to stop.
		reportOrDiscard().Step("config-admin-agent from a previous run is still active, stopping it...")
		if result := configRunStopAgentFn(); result.Success() {
			reportOrDiscard().Ok("config-admin-agent stopped.")
		} else {
			reportOrDiscard().Fail("Failed to stop config-admin-agent.")
			if result.Err != nil {
				reportOrDiscard().Fault("Error message: %s", result.Err.Error())
			}
			if result.ExitCode != common.ErrSuccess {
				reportOrDiscard().Fault("Exit code: %s", strconv.Itoa(result.ExitCode))
			}
		}
	}
	if gameRunningFn() {
		atomicExitCode.Store(int32(launcher.ErrGameAlreadyRunning))
		return
	}
	/*
		Ensure:
		* No running config-admin-agent nor agent processes
		* Any previous changes are reverted
	*/
	// This is a teardown, not a setup step: what it undoes is what the previous
	// run left behind, not anything this one has done. Labelling it as execution
	// work is what made "Cleaning up" read as part of the startup, and calling
	// both of them "Teardown" is what made the two look like a bug.
	launcher.ActivePresenter().Section("Initial teardown")
	reportOrDiscard().Step("Cleaning up (if needed)...")
	configKillAgentFn()
	if err = commonLoggerFileLoggerBufferFn("config_revert_initial", func(writer io.Writer) {
		launcherCommon.ConfigRevert("", commonLogger.FileLogger.Folder(), false, writer, func(options *exec.Options) {
			commonLogger.Println("run config revert", options.String())
		}, executor.RunRevert)
	}); err != nil {
		atomicExitCode.Store(int32(common.ErrFileLog))
		return
	}
	if canTrustCertificate == "auto" {
		if runtime.GOOS == "darwin" {
			canTrustCertificate = "user"
		} else {
			canTrustCertificate = "local"
		}
	}
	customHostFile := slices.ContainsFunc(cfg.Client.Args, func(s string) bool {
		return strings.Contains(s, "{HostFilePath}")
	})
	customCertFile := slices.ContainsFunc(cfg.Client.Args, func(s string) bool {
		return strings.Contains(s, "{CertFilePath}")
	})
	cfgFlushCacheOpts := newConfigFlushCacheOptionsFn(canAddHost, canTrustCertificate, customHostFile, customCertFile)
	if cfgFlushCacheOpts != nil {
		if result := cfgFlushCacheOpts.RunFlushCache(); !result.Success() {
			atomicExitCode.Store(int32(launcher.ErrFlushCache))
			return
		}
	}
	if _, proc, localErr := commonProcessProcessFn(executablesNativeFileNameFn(false, executables.Server)); localErr == nil && proc != nil {
		reportOrDiscard().Warn("Server is already running, If you did not start it manually, kill the server process using the task manager and execute the launcher again.")
	}
	if err = commonLoggerFileLoggerBufferFn("revert_command_initial", func(writer io.Writer) {
		if err = executor.RunRevertCommand(writer, func(options *exec.Options) {
			commonLogger.Println("run revert command", options.String())
		}); err != nil {
			reportOrDiscard().Fail("Failed to run revert command.")
			reportOrDiscard().Fault("Error message: %s", err.Error())
		}
	}); err != nil {
		atomicExitCode.Store(int32(common.ErrFileLog))
		return
	}
	logger.WriteFileLog(gameId, "post initial cleanup")
	if len(revertCommand) > 0 {
		if err = launcherCommon.RevertCommandStore.Store(revertCommand); err != nil {
			reportOrDiscard().Fail("Failed to store revert command")
			atomicExitCode.Store(int32(launcher.ErrInvalidRevertCommand))
			return
		}
	}
	// Everything from here on is this run's own work: the previous run's leftovers
	// are gone, and what follows is what the game will actually see.
	// A check per phase, at the points where the run is about to start changing
	// the machine. Checking here rather than inside the shared operations means
	// one place decides where a stop is safe, and every step after the check
	// runs to completion: a stop never lands halfway through a revert.
	if cancelled("setting up") {
		return
	}
	launcher.ActivePresenter().Section("Execution")
	// Setup
	reportOrDiscard().Step("Setting up...")
	if len(setupCommand) > 0 {
		reportOrDiscard().Step("Running setup command '%s' and waiting for it to exit...", cfg.Config.SetupCommand)
		result := configRunSetupCommandFn(setupCommand)
		if !result.Success() {
			if result.Err != nil {
				reportOrDiscard().Fault("Error: %s", result.Err)
			}
			if result.ExitCode != common.ErrSuccess {
				reportOrDiscard().Fault("Exit code: %d.", result.ExitCode)
			}
			atomicExitCode.Store(int32(launcher.ErrSetupCommand))
			return
		}
	}
	var serverIP string
	if serverStart == "auto" {
		announcePorts := cfg.Server.AnnouncePorts
		ports := mapset.NewThreadUnsafeSetWithSize[uint16](len(announcePorts))
		for _, portInt := range announcePorts {
			ports.Add(uint16(portInt))
		}
		multicastIPsStr := cfg.Server.AnnounceMulticastGroups
		multicastIPs := mapset.NewThreadUnsafeSetWithSize[netip.Addr](len(multicastIPsStr))
		for _, str := range multicastIPsStr {
			if IP, localErr := netipParseAddrFn(str); localErr == nil && IP.Is4() && IP.IsMulticast() {
				multicastIPs.Add(IP)
			} else {
				reportOrDiscard().Fail("Invalid multicast group \"%s\"", str)
				atomicExitCode.Store(int32(launcher.ErrAnnouncementMulticastGroup))
				return
			}
		}
		serverId, selectedServerIp := discoverServersFn(
			gameId,
			cfg.Server.SingleAutoSelect,
			multicastIPs,
			ports,
		)
		if serverId != uuidNilFn() {
			serverIP = selectedServerIp.String()
			serverStart = "false"
			serverArgsValues.Id = serverId.String()
			if serverStop == "auto" && (!isAdmin || runtime.GOOS == "windows") {
				serverStop = "false"
			}
		} else {
			serverStart = "true"
			if serverStop == "auto" {
				serverStop = "true"
			}
		}
	}
	if serverStart == "false" {
		if serverStop == "true" {
			reportOrDiscard().Warn("serverStart is false. Ignoring serverStop being true.")
		}
		if serverIP == "" {
			if serverHost == "" {
				reportOrDiscard().Fail("serverStart is false. serverHost must be fulfilled as it is needed to know which host to connect to.")
				atomicExitCode.Store(int32(launcher.ErrInvalidServerHost))
				return
			}
			if addr, localErr := netipParseAddrFn(serverHost); localErr == nil && addr.Is6() {
				reportOrDiscard().Fail("serverStart is false. serverHost must be fulfilled with a host or Ipv4 address.")
				atomicExitCode.Store(int32(launcher.ErrInvalidServerHost))
				return
			}
			if id, measuredServerIPAddrs, data := serverFilterServerIPsFn(
				uuidNilFn(),
				serverHost,
				gameId,
				commonNetIPSliceToNetIPSetFn(commonStringSliceToNetIPSliceFn(commonHostOrIpToIpsFn(serverHost))),
			); data == nil {
				reportOrDiscard().Fail("serverStart is false. Failed to resolve serverHost to a valid and reachable IP.")
				atomicExitCode.Store(int32(launcher.ErrInvalidServerHost))
				return
			} else {
				serverIP = measuredServerIPAddrs[0].Ip.String()
				serverArgsValues.Id = id.String()
			}
		}
	} else {
		if logRoot := commonLogger.FileLogger.Folder(); logRoot != "" {
			serverArgsValues.Log = true
			serverArgsValues.LogRoot = logRoot
			serverArgsValues.Flatlog = true
			serverArgsValues.Deterministic = true
		}
		battleServerRequired := configBattleServerRequiredFn(executer)
		if battleServerRequired && serverArgsValues.StartBsIfMissing == "false" && battleServerManagerRun == "false" {
			reportOrDiscard().Warn("This game needs a Battle Server to be started but you don't allow to start one, make sure you have one running and the server configured.")
		}
		runBattleServerManager := battleServerManagerRun == "true" || (battleServerManagerRun == "required" && battleServerRequired)
		if cfg.Server.Start == launcher.ModeAuto && !cfg.Server.StartWithoutConfirmation {
			var str string
			if runBattleServerManager {
				str = "No servers were found. Start a battle server (if needed) and then start the game server?"
			} else {
				str = "No servers were found. Start the game server?"
			}
			if !launcher.ActiveDialog().ConfirmStartServer(str, promptInput()) {
				reportOrDiscard().Fail("Canceled starting the server.")
				atomicExitCode.Store(int32(launcher.ErrServerStartCanceled))
				return
			}
		}
		if cancelled("starting the server") {
			return
		}
		serverExecutablePath := serverGetExecutablePathFn(serverExecutable)
		if serverExecutablePath == "" {
			reportOrDiscard().Fail("Cannot find server executable path. Set it manually in Server.Executable.")
			atomicExitCode.Store(int32(launcher.ErrServerExecutable))
			return
		}
		if serverExecutable != serverExecutablePath {
			reportOrDiscard().Ok("Found server executable path: %s", serverExecutablePath)
		}
		if ec := serverGenerateCertsFn(serverExecutablePath, canTrustCertificate != "false"); ec != common.ErrSuccess {
			atomicExitCode.Store(int32(ec))
			return
		}
		if runBattleServerManager {
			values, flags := bsManagerStartFlagSetFn(nil)
			if err = flags.Parse(battleServerManagerArgs); err != nil {
				reportOrDiscard().Fail("Failed to parse battle-server-manager executable arguments")
				atomicExitCode.Store(int32(launcher.ErrInvalidServerBattleServerManagerArgs))
				return
			}
			ec := configRunBattleServerManagerFn(
				battleServerManagerExecutable,
				flags,
				values,
				serverStop == "true",
			)
			if ec != common.ErrSuccess {
				atomicExitCode.Store(int32(ec))
				return
			}
		}
		var ec int
		ec, serverIP = configStartServerFn(serverExecutablePath, serverFlags, serverArgsValues, serverStop == "true")
		if ec != common.ErrSuccess {
			atomicExitCode.Store(int32(ec))
			return
		}
	}
	serverCertificate := serverReadCACertFn(serverIP)
	if serverCertificate == nil {
		reportOrDiscard().Fail("Failed to read certificate from %s. Try to access it with your browser and checking the certificate.", serverIP)
		atomicExitCode.Store(int32(launcher.ErrReadCert))
		return
	}
	// Past this point the run stops hosts, installs certificates and launches the
	// game, and each of those is something the user would have to undo by hand.
	// It is the last moment where stopping is still free.
	if cancelled("changing the machine") {
		return
	}
	atomicExitCode.Store(int32(configMapHostsFn(gameId, serverIP, macOsExclusiveMappings, canAddHost, customHostFile)))
	if atomicExitCode.Load() != int32(common.ErrSuccess) {
		return
	}
	logger.WriteFileLog(gameId, "post host mapping")
	atomicExitCode.Store(int32(configAddCertFn(gameId, uuidMustParseFn(serverArgsValues.Id), serverCertificate, canTrustCertificate, customCertFile, macOsExclusiveMappings)))
	if atomicExitCode.Load() != int32(common.ErrSuccess) {
		return
	}
	logger.WriteFileLog(gameId, "post add cert")
	atomicExitCode.Store(int32(configIsolateUserDataFn(isolateMetadata, isolateProfiles, isolationPath)))
	if atomicExitCode.Load() != int32(common.ErrSuccess) {
		return
	}
	logger.WriteFileLog(gameId, "post isolate user data")
	if gamePath != "" {
		atomicExitCode.Store(int32(configAddCACertToGameFn(gameId, uuidMustParseFn(serverArgsValues.Id), serverCertificate, configGamePathToGameCertPathFn(executer, gamePath), gameCaCertPath, cfg.Config.Certificate.CanTrustInGame, macOsExclusiveMappings)))
		if atomicExitCode.Load() != int32(common.ErrSuccess) {
			return
		}
		logger.WriteFileLog(gameId, "post add game cert")
	}
	atomicExitCode.Store(int32(configLaunchAgentAndGameFn(executer, customExecutor, cfg.Client.Args, canTrustCertificate, canBroadcastBattleServer, isolationPath)))
	return
}

// configErr is the failure behind a nil configuration. It is kept beside the
// configuration rather than returned with it because the loader is injected, and
// its signature is what the tests have always injected.
var configErr error

func initConfig(fs *pflag.FlagSet) *launcher.Configuration {
	configErr = nil
	cfg, loaded, err := launcher.LoadConfig(fs, optionStorage(), reportOrDiscard())
	usedConfigFile = loaded.MainFile
	filesToPrint = loaded.FilesToPrint
	if err == nil {
		return cfg
	}
	// The loader reports the problem but not how this frontend says it, which is
	// the same split as everywhere else: the shared code does not know whether
	// this is a terminal or a window.
	configErr = err
	var ee *launcher.ExitError
	if !errors.As(err, &ee) {
		reportOrDiscard().Fail("%s", err.Error())
		return nil
	}
	if ee.Game {
		reportOrDiscard().Fail("Error parsing game config file: %s:%s", ee.Path, ee.Err.Error())
	} else if fileErr, ok := errors.AsType[*common.KoanfFileLoadError](ee.Err); ok {
		commonLogger.Println("Error parsing config file:", fileErr.Path+":", fileErr.Err.Error())
	} else {
		commonLogger.Println("Error loading config:", ee.Err.Error())
	}
	return nil
}

// orNone names an absent config file rather than leaving the summary with a
// blank next to a label: "none" says the run used defaults on purpose, and an
// empty cell reads as something that failed to load.
func orNone(path string) string {
	if path == "" {
		return "none"
	}
	return path
}
