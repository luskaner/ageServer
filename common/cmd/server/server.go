package server

import (
	"fmt"
	"strings"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type Values struct {
	*cmd.LogRootValues
	CanUseInternet         bool
	GameIds                []string
	Id                     string
	CfgFile                string
	Flatlog                bool
	Deterministic          bool
	Log                    bool
	GeneratePlatformUserId bool
	Announce               string
	AnnouncePort           int
	AnnounceMulticast      string
	AnnounceMulticastGroup string
	StartBsIfMissing       string
}

func SingleFlagSet(version string, configPaths []string, runFn func(*pflag.FlagSet) (err error, exitCode int)) (values *Values, singleFs *cmd.SingleFlagSet) {
	singleFs = cmd.NewSingleFlagSet(runFn, version)
	values = &Values{
		LogRootValues: &cmd.LogRootValues{},
	}
	flags := singleFs.Fs()
	flags.StringVar(&values.CfgFile, "config", "", fmt.Sprintf(`config file (default config.toml in %s directories)`, strings.Join(configPaths, ", ")))
	flags.BoolVar(&values.CanUseInternet, "internet", true, "Whether to allow the 'server' to use the internet to look up official domains and for authentication/public IP resolution. If false, only the statically known domains will be used.")
	flags.StringVarP(&values.Announce, "announce", "a", "true", "Respond to discove 'server' in LAN. Disabling this will not allow launchers to discover it and will require specifying the host")
	flags.IntVarP(&values.AnnouncePort, "announcePort", "p", common.AnnouncePort, "Port to respond to discovery requests. If changed, the 'launcher's will need to specify the port in Server.AnnouncePorts")
	flags.StringVarP(&values.AnnounceMulticast, "announceMulticast", "m", "true", "Whether to respond to discovery queries using Multicast.")
	flags.StringVarP(&values.AnnounceMulticastGroup, "announceMulticastGroup", "i", common.AnnounceMulticastGroup, "Multicast address to respond to discovery queries if 'announce' is enabled.")
	flags.BoolVar(&values.Log, "log", false, "Whether to log more info to a file. Enable it for errors.")
	flags.BoolVar(&values.Flatlog, "flatLog", false, "Whether to log in a flat structure in --logRoot. Only applicable if --log is passed.")
	flags.BoolVar(&values.Deterministic, "deterministic", false, "Whether to be as deterministic as possible.")
	cmd.GamesVarCommand(flags, &values.GameIds)
	cmd.LogRootCommand(flags, &values.LogRoot)
	flags.BoolVarP(&values.GeneratePlatformUserId, "generatePlatformUserId", "g", false, "Generate the Platform User Id based on the user's IP.")
	flags.StringVar(&values.Id, "id", "", "Server instance ID to identify it.")
	flags.StringVar(&values.StartBsIfMissing, "startBsIfMissing", "true", "Embedded battle server mode for all enabled games (true, false, required)")
	return
}
