package agent

import (
	"runtime"

	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type Values struct {
	*cmd.GameIdValues
	*cmd.LogRootValues
	ServerExecutable              string
	ProcessNames                  []string
	BattleServerLANRebroadcast    bool
	BattleServerManagerExecutable string
	BattleServerRegion            string
	BaseDataPath                  string
}

func SingleFlagSet(version string, runFn func(*pflag.FlagSet) (err error, exitCode int)) (values *Values, singleFs *cmd.SingleFlagSet) {
	singleFs = cmd.NewSingleFlagSet(runFn, version)
	values = &Values{
		GameIdValues:  &cmd.GameIdValues{},
		LogRootValues: &cmd.LogRootValues{},
	}
	flags := singleFs.Fs()
	cmd.LogRootCommand(flags, values.LogRootRef())
	cmd.GameVarCommand(flags, values.GameIdRef())
	flags.StringVar(&values.ServerExecutable, "serverExecutable", "", "Path to the Server executable to for stopping it after game closes.")
	flags.StringVar(&values.BattleServerManagerExecutable, "bsManagerExecutable", "", "Path to the standalone Battle Server Manager executable to monitor.")
	flags.StringVar(&values.BattleServerRegion, "bsRegion", "", "Region of the standalone battle server for stopping it after game closes.")
	if runtime.GOOS == "windows" {
		flags.BoolVar(&values.BattleServerLANRebroadcast, "bsLanRebroadcast", false, "Whether to rebroadcast LAN packets from the standalone Battle Server to the local network.")
	}
	flags.StringSliceVar(&values.ProcessNames, "processes", nil, "Process names to monitor for game start/stop events.")
	flags.StringVar(&values.BaseDataPath, "baseDataPath", "", "Base path for user data.")
	return
}
