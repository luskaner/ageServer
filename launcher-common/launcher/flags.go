package launcher

import (
	"fmt"
	"net/netip"
	"runtime"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/common"
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/paths"
	"github.com/spf13/pflag"
)

// autoValue is the default of every option that has a third answer of "decide
// for me", which is what lets a run work without anyone answering anything.
const autoValue = "auto"

// ConfigPaths are the directories searched for a config file when none was
// named, most specific first.
func ConfigPaths() []string { return []string{paths.ResourcesDir, "."} }

// Values are the options the shared launcher reads by name.
//
// They are pointers rather than fields because a flag set can only write into
// storage the caller owns. Each frontend owns its own: the console launcher
// points these at its flag variables, a graphical one at whatever holds the
// values its controls edited, and neither has to know how the other stores
// them.
//
// Everything else is read back out of the flag set by name when the
// configuration is loaded, so a frontend that wants a value does not need it
// here.
type Values struct {
	ConfigFile     *string
	GameConfigFile *string
	GameID         *string
	Output         *string
}

// BindFlags registers every launcher option on fs, writing into v.
//
// The option names, shorthands, defaults and help text live here rather than in
// the console launcher so that the two cannot drift: a graphical frontend
// registering the same options on a flag set of its own gets the same
// configuration out of the same files, and a renamed option changes in one
// place.
//
// A nil target is rejected rather than ignored. A frontend that left one out
// would get a flag that parses into nothing, which fails later and further from
// the mistake than this does.
func BindFlags(fs *pflag.FlagSet, v Values) error {
	for name, target := range map[string]any{
		"ConfigFile":     v.ConfigFile,
		"GameConfigFile": v.GameConfigFile,
		"GameID":         v.GameID,
		"Output":         v.Output,
	} {
		if target == nil {
			return fmt.Errorf("launcher: %s target is nil", name)
		}
	}

	configPaths := ConfigPaths()
	fs.StringVar(v.ConfigFile, "config", "", fmt.Sprintf(`config file (default config.toml in %s directories)`, strings.Join(configPaths, ", ")))
	fs.StringVar(v.GameConfigFile, "gameConfig", "", fmt.Sprintf(`Game config file (default config.game.toml in %s directories)`, strings.Join(configPaths, ", ")))
	fs.BoolP("canUseDialogs", "d", true, `Whether to ask the interactive questions (which server to use, and whether to start one) in a graphical window instead of in the console, "false" always asks in the console. It always falls back to the console if graphical dialogs are unavailable.`)
	fs.StringVar(v.Output, "output", autoValue, `How much to decorate the console output with, "auto" picks whatever the terminal can show without turning characters into boxes, "color" asks for colour but still degrades to plain text if the console cannot do it, "ascii" guarantees plain ASCII text. It also reads the AGE_LANSERVER_OUTPUT environment variable.`)
	fs.Bool("log", false, "Whether to log more info to a file. Enable it for errors.")
	fs.Bool("internet", true, "Whether or not the launcher may use the internet to look up official domains. If false, only the statically known domains will be used for the hosts file, certificates and logs. If true and there is no internet connectivity the launcher will still work with the statically known domains.")
	fs.StringP("canAddHost", "t", "true", "Add a local dns entry if it's needed to connect to the server with the official domain. Including to avoid receiving that it's on maintenance. Ignored if clientExeArgs contains '{HostFilePath}'. Will require admin privileges.")
	canTrustCertificateStr := `Trust the certificate of the server if needed. "false"`
	if runtime.GOOS != "linux" {
		canTrustCertificateStr += `, "user"`
	}
	canTrustCertificateStr += ` or local (will require admin privileges). Ignored if clientExeArgs contains '{CertFilePath}'.`
	fs.StringP("canTrustCertificate", "c", "local", canTrustCertificateStr)
	if runtime.GOOS == "windows" {
		fs.StringP("canBroadcastBattleServer", "b", autoValue, `Whether or not to broadcast the game BattleServer to all interfaces in LAN (not just the most priority one)`)
	}
	var pathNamesInfo string
	if runtime.GOOS == "windows" {
		pathNamesInfo += " Path names need to use double backslashes within single quotes or be within double quotes."
	}
	commonCmd.GameVarCommand(fs, v.GameID)
	fs.StringP("isolateMetadata", "m", "required", "Isolate the metadata cache of the game, otherwise, it will be shared. Not compatible with AoE:DE. If 'required' it will resolve to 'true' if using the official launcher, 'false' otherwise.")
	fs.StringP("isolateProfiles", "p", "required", "Isolate the user's profile of the game, otherwise, it will be shared. If 'required' it will resolve to 'true' if using the official launcher, 'false' otherwise.")
	fs.String("setupCommand", "", `Executable to run (including arguments) to run first after the "Setting up..." line. The command must return a 0 exit code to continue. If you need to keep it running spawn a new separate process. You may use environment variables.`+pathNamesInfo)
	fs.String("revertCommand", "", `Executable to run (including arguments) to run after setupCommand, game has exited and everything has been reverted. It may run before if there is an error. You may use environment variables.`+pathNamesInfo)
	fs.StringP("serverStart", "a", autoValue, `Start the server if needed, "auto" will start a server if one is not already running, "true" (will start a server regardless if one is already running), "false" (will require an already running server).`)
	fs.StringP("serverStop", "o", autoValue, `Stop the server if started, "auto" will stop the server if one was started, "false" (will not stop the server regardless if one was started), "true" (will not stop the server even if it was started).`)
	fs.StringSliceP("serverAnnouncePorts", "n", []string{strconv.Itoa(common.AnnouncePort)}, `Announce ports to listen to. If not including the default port, default configured 'servers' will not get discovered.`)
	fs.StringSliceP("serverAnnounceMulticastGroups", "g", []string{common.AnnounceMulticastGroup}, `Announce multicast groups to join. If not including the default group, default configured 'servers' will not get discovered via Multicast.`)
	fs.StringP("server", "s", "", `Hostname of the server to connect to. If not absent, serverStart will be assumed to be false. Ignored otherwise`)
	fs.Bool("serverSingleAutoSelect", false, `Auto-select the server when a single one is discovered.`)
	serverExe := executables.NativeFileName(false, executables.Server)
	fs.StringP("serverPath", "z", autoValue, fmt.Sprintf(`The executable path of the server, "auto", will be try to execute in this order "./%s/%s", "../%s" and finally "../%s/%s", otherwise set the path (relative or absolute).`, executables.Server, serverExe, serverExe, executables.Server, serverExe))
	fs.StringP("serverPathArgs", "r", "", `The arguments to pass to the server executable if starting it. Execute the server help flag for available arguments. You may use environment variables.`+pathNamesInfo)
	clientExeTip := `The type of game client or the path. "auto" will use `
	if runtime.GOOS != "darwin" {
		clientExeTip += "Steam"
	}
	unixClientExeTip := `Steam (CrossOver) and then the Steam (Wine) one if found`
	if runtime.GOOS == "windows" {
		clientExeTip += ` and then the Xbox one if found`
	}
	if runtime.GOOS == "linux" {
		clientExeTip += `, `
	}
	if runtime.GOOS != "windows" {
		clientExeTip += unixClientExeTip
	}
	clientExeTip += `. Use a path to the game launcher,`
	if runtime.GOOS != "darwin" {
		clientExeTip += ` "steam"`
	}
	if runtime.GOOS == "linux" {
		clientExeTip += `,`
	}
	if runtime.GOOS != "windows" {
		clientExeTip += ` "steam_crossover" or "steam_wine"`
	}
	if runtime.GOOS == "windows" {
		clientExeTip += ` or "msstore"`
	}
	clientExeTip += " to use the default launcher."
	fs.StringP("clientExe", "l", autoValue, clientExeTip)
	fs.StringP("clientExeArgs", "i", "", "The arguments to pass to the client launcher if it is custom. You may use environment variables and '{HostFilePath}'/'{CertFilePath}'/'{SigKeyFilePath}' replacement variables."+pathNamesInfo)

	return nil
}

// defaultLayers are the built-in defaults, below every file and every flag.
//
// Every option that has a value the run can proceed with has an entry here, so a
// launcher with no config file and no flags at all still produces a runnable
// configuration instead of an empty struct full of zero values.
func defaultLayers() map[string]any {
	defaults := map[string]any{
		"Config.CanUseDialogs":                      true,
		"Config.CanAddHost":                         "true",
		"Config.Certificate.CanTrustInPc":           "local",
		"Config.Certificate.CanTrustInGame":         true,
		"Config.CanBroadcastBattleServer":           autoValue,
		"Config.CanUseInternet":                     true,
		"Config.Log":                                false,
		"Client.Isolation.Metadata":                 "required",
		"Client.Isolation.Profiles":                 "required",
		"Config.SetupCommand":                       []string{},
		"Config.RevertCommand":                      []string{},
		"Client.Executable":                         autoValue,
		"Client.ExecutableArgs":                     []string{},
		"Client.Path":                               autoValue,
		"Server.Start":                              autoValue,
		"Server.Stop":                               autoValue,
		"Server.SingleAutoSelect":                   false,
		"Server.StartWithoutConfirmation":           false,
		"Server.Executable":                         autoValue,
		"Server.ExecutableArgs":                     []string{"-e", "{Game}", "--id", "{Id}"},
		"Server.Host":                               netip.IPv4Unspecified().String(),
		"Server.AnnouncePorts":                      []int{common.AnnouncePort},
		"Server.AnnounceMulticastGroups":            []string{common.AnnounceMulticastGroup},
		"Server.BattleServerManager.Run":            "false",
		"Server.BattleServerManager.Executable":     autoValue,
		"Server.BattleServerManager.ExecutableArgs": []string{"-e", "{Game}", "-r"},
	}
	for g := range game.SupportedGames.Iter() {
		defaults[fmt.Sprintf("Games.%s.Hosts", g)] = []string{netip.IPv4Unspecified().String()}
	}
	return defaults
}

// flagBindings maps an option name onto the configuration key it fills.
//
// The two disagree often enough that guessing is not an option: the flag is
// named after what it does on the command line and the key after where it ends
// up. An option missing from this map is still readable from the flag set, it
// just does not reach the configuration, which is how a typo stays invisible
// until a run behaves differently from what the help promised.
func flagBindings() map[string]string {
	return map[string]string{
		"canUseDialogs":                 "Config.CanUseDialogs",
		"canAddHost":                    "Config.CanAddHost",
		"canTrustCertificate":           "Config.Certificate.CanTrustInPc",
		"canBroadcastBattleServer":      "Config.CanBroadcastBattleServer",
		"internet":                      "Config.CanUseInternet",
		"log":                           "Config.Log",
		"isolateMetadata":               "Client.Isolation.Metadata",
		"isolateProfiles":               "Client.Isolation.Profiles",
		"setupCommand":                  "Config.SetupCommand",
		"revertCommand":                 "Config.RevertCommand",
		"serverStart":                   "Server.Start",
		"serverStop":                    "Server.Stop",
		"serverSingleAutoSelect":        "Server.SingleAutoSelect",
		"serverAnnouncePorts":           "Server.AnnouncePorts",
		"serverAnnounceMulticastGroups": "Server.AnnounceMulticastGroups",
		"server":                        "Server.Host",
		"serverPath":                    "Server.Executable",
		"serverPathArgs":                "Server.ExecutableArgs",
		"clientExe":                     "Client.Executable",
		"clientExeArgs":                 "Client.ExecutableArgs",
	}
}
