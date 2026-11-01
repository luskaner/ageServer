package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/spf13/pflag"
)

// ExitError is a failure that comes with the code the run should end on.
//
// The launcher used to decide this by calling os.Exit from wherever it noticed,
// which is fine for a console program and useless for anything that keeps
// running: there is nothing to return an exit code to. Carrying the code with
// the error lets the console exit with exactly what it exited with before and a
// window decide for itself what to show and what to do next.
type ExitError struct {
	// Code is the process exit code the console launcher used.
	Code int
	// Err is what went wrong.
	Err error
	// Game is set when the failure came from the per-game config file rather
	// than the main one, because the two are reported differently: one names
	// the file it could not read, the other is about the configuration as a
	// whole.
	Game bool
	// Path is the file being read when it failed.
	Path string
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode returns the exit code an ExitError carries, and false for any other
// error. A caller that only wants the code can ignore the message and let the
// frontend decide how to say it.
func ExitCode(err error) (int, bool) {
	if ee, ok := errors.AsType[*ExitError](err); ok {
		return ee.Code, true
	}
	return 0, false
}

// Loaded records where a run's configuration came from.
//
// The files are part of the answer rather than a side effect: a run that behaves
// unexpectedly is usually explained by which file it actually read, and a
// frontend has to be able to show that without guessing.
type Loaded struct {
	// MainFile is the config file that was loaded, empty when none was found
	// and the defaults were used.
	MainFile string
	// GameFile is the per-game config file that was loaded, empty when none
	// was found.
	GameFile string
	// FilesToPrint are the files to name in the run summary, in load order.
	FilesToPrint []string
}

// LoadConfig resolves the configuration for a run.
//
// It is the same resolution the console launcher has always done, in the same
// order: built-in defaults, then the environment, then the first config file
// that exists, then the flags. Those layers are why a config file and a flag can
// disagree and both be right.
//
// v carries the options this function needs to resolve which files to read, and
// fs supplies the rest by name. r receives the two things worth saying out loud
// while it happens: that no config file was found when one was named, and a
// config file that could not be parsed.
//
// A failure comes back as an *ExitError rather than an exit.
func LoadConfig(fs *pflag.FlagSet, v Values, r Reporter) (*Configuration, Loaded, error) {
	var loaded Loaded
	if r == nil {
		r = Discard{}
	}
	k := koanf.New(".")

	mainfileCandidates := ConfigPaths2File("config.toml", deref(v.ConfigFile))
	mainFile, err := common.LoadKoanfLayers(k, defaultLayers(), mainfileCandidates, toml.Parser(), fs, flagBindings(), executables.Launcher)
	if err != nil {
		// Reported without decoration, the way it always was: this one is not
		// a step that failed, it is the run failing to start, and there is no
		// step to hang a marker on.
		return nil, loaded, &ExitError{Code: common.ErrConfigParse, Err: err}
	}
	loaded.MainFile = mainFile
	if deref(v.ConfigFile) != "" && mainFile == "" {
		r.Warn("No config file found, using defaults.")
	}
	if mainFile != "" {
		// Not printed here: the summary at the top of the run already names
		// this file, and two copies of the same path is one more thing to keep
		// in sync.
		loaded.FilesToPrint = append(loaded.FilesToPrint, mainFile)
	}

	gameFileCandidates := ConfigPaths2File(fmt.Sprintf("config.%s.toml", deref(v.GameID)), deref(v.GameConfigFile))
	gameFile, err := common.LoadKoanfLayers(k, map[string]any{}, gameFileCandidates, toml.Parser(), fs, nil, executables.Launcher)
	if err != nil {
		// A missing file is not a failure: not every game ships a config of its
		// own. Anything else is, and it is the one case where the file being
		// read is the most useful thing to name.
		if _, ok := errors.AsType[*common.KoanfFileLoadError](err); !ok {
			return nil, loaded, &ExitError{Code: ErrGameConfigParse, Err: err, Game: true, Path: deref(v.GameConfigFile)}
		}
	} else {
		loaded.GameFile = gameFile
		loaded.FilesToPrint = append(loaded.FilesToPrint, gameFile)
		// Written back so a frontend that asked for a file knows which one was
		// actually used.
		if v.GameConfigFile != nil {
			*v.GameConfigFile = gameFile
		}
	}

	var c Configuration
	if err := k.Unmarshal("", &c); err != nil {
		return nil, loaded, &ExitError{Code: common.ErrConfigParse, Err: err}
	}
	return &c, loaded, nil
}

// ConfigPaths2File builds the list of candidates for a config file: the one that
// was named, or the same name in every directory searched.
func ConfigPaths2File(name string, explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	var candidates []string
	for _, dir := range ConfigPaths() {
		candidates = append(candidates, filepath.Join(dir, name))
	}
	return candidates
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// The values each option that is not free text accepts.
//
// They are spelled once, here, because two things need them and neither can work
// them out for itself: the validators below, which reject a configuration that
// cannot produce a run, and a frontend drawing the control for the option. A
// window that listed them itself would drift from the run it is configuring, and
// the drift would only show up as a run refusing a value its own form had
// offered.
//
// Each accessor hands back a set of its own, because a frontend may well want to
// take one apart, and a shared one it emptied would leave the next run rejecting
// everything.
func values(vs ...string) mapset.Set[string] { return mapset.NewThreadUnsafeSet[string](vs...) }

// list spells a set of values in one fixed order, so that a message naming what
// an option accepts reads the same on every run. A set has no order of its own.
func list(vs mapset.Set[string]) string {
	out := vs.ToSlice()
	slices.Sort(out)
	return strings.Join(out, "/")
}

// AutoTrueFalseValues are the values of an option that may also be left to the
// run: auto, true or false.
func AutoTrueFalseValues() mapset.Set[string] { return values(autoValue, ModeTrue, ModeFalse) }

// CanTrustCertificateValues are the values of canTrustCertificate.
//
// There is no "user" on linux: the per-user trust store does not exist there, so
// a certificate is trusted in the system one or not at all, and offering a value
// the run would reject would be worse than not offering it.
func CanTrustCertificateValues() mapset.Set[string] {
	validValues := values(autoValue, ModeFalse, "user", "local")
	if runtime.GOOS == "linux" {
		validValues.Remove("user")
	}
	return validValues
}

// CanBroadcastBattleServerValues are the values of canBroadcastBattleServer,
// which has no answer of its own beyond not doing it.
func CanBroadcastBattleServerValues() mapset.Set[string] { return values(autoValue, ModeFalse) }

// ServerStopValues are the values of serverStop.
//
// "false" is missing when the run is not on windows and is already
// administrator: a server it could stop only by asking for privileges it does
// not have is not an answer the run can act on.
func ServerStopValues(nonWindowsAdmin bool) mapset.Set[string] {
	validValues := values(autoValue, ModeTrue, ModeFalse)
	if nonWindowsAdmin {
		validValues.Remove(ModeFalse)
	}
	return validValues
}

// RequiredTrueFalseValues are the values of the options that are required to be
// either true or false unless someone says they will be decided.
func RequiredTrueFalseValues() mapset.Set[string] { return values(ModeTrue, ModeFalse, "required") }

// The validators reject a configuration that cannot produce a working run, and
// they return the exit code to end on rather than ending it: same reason as
// ExitError, one layer up.
//
// They share one r because they share one moment in the run: a config that came
// off disk or a flag that was typed, checked before anything is set up, so that
// nothing has to be torn down afterwards.
//
// Each one reads the same accessor its frontend read when it offered the values,
// so the two cannot disagree about what an option accepts.

func ValidateCanTrustCertificate(r Reporter, canTrustCertificate string) (exitCode int) {
	if validValues := CanTrustCertificateValues(); !validValues.Contains(canTrustCertificate) {
		r.Fail("Invalid value for canTrustCertificate (%s): %s", list(validValues), canTrustCertificate)
		return ErrInvalidCanTrustCertificate
	}
	return common.ErrSuccess
}

func ValidateCanBroadcastBattleServer(r Reporter, canBroadcastBattleServer string) (exitCode int) {
	if validValues := CanBroadcastBattleServerValues(); !validValues.Contains(canBroadcastBattleServer) {
		r.Fail("Invalid value for canBroadcastBattleServer (%s): %s", list(validValues), canBroadcastBattleServer)
		return ErrInvalidCanBroadcastBattleServer
	}
	return common.ErrSuccess
}

func ValidateServerStartValue(r Reporter, serverStart string) (exitCode int) {
	if validValues := AutoTrueFalseValues(); !validValues.Contains(serverStart) {
		r.Fail("Invalid value for serverStart (%s): %s", list(validValues), serverStart)
		return ErrInvalidServerStart
	}
	return common.ErrSuccess
}

func ValidateServerStopValue(r Reporter, serverStop string, nonWindowsAdmin bool) (exitCode int) {
	if validValues := ServerStopValues(nonWindowsAdmin); !validValues.Contains(serverStop) {
		r.Fail("Invalid value for serverStop (%s): %s", list(validValues), serverStop)
		return ErrInvalidServerStop
	}
	return common.ErrSuccess
}

func ValidateRequiredTrueFalse(r Reporter, value string, name string, validValues mapset.Set[string]) (exitCode int) {
	if !validValues.Contains(value) {
		r.Fail("Invalid value for %s (%s): %s", name, list(validValues), value)
		switch name {
		case "Server.BattleServerManager.Run":
			return ErrInvalidServerBattleServerManagerRun
		case "Client.Isolation.Metadata":
			return ErrInvalidIsolateMetadata
		case "Client.Isolation.Profiles":
			return ErrInvalidIsolateProfiles
		}
	}
	return common.ErrSuccess
}
