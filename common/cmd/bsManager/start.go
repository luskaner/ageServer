package bsManager

import (
	"fmt"
	"strings"

	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type StartValues struct {
	*cmd.GameIdValues
	*cmd.LogRootValues
	GameCfgFile   string
	HideWindow    bool
	Force         bool
	NoErrExisting bool
}

func StartFlagSet(configPaths []string) (values *StartValues, flags *pflag.FlagSet) {
	values = &StartValues{
		GameIdValues:  &cmd.GameIdValues{},
		LogRootValues: &cmd.LogRootValues{},
	}
	flags = pflag.NewFlagSet("start", pflag.ContinueOnError)
	cmd.LogRootCommand(flags, values.LogRootRef())
	cmd.GameVarCommand(flags, values.GameIdRef())
	flags.StringVar(&values.GameCfgFile, "gameConfig", "", fmt.Sprintf(`Game config file (default config.game.toml in %s directories)`, strings.Join(configPaths, ", ")))
	flags.BoolVarP(&values.HideWindow, "hideWindow", "w", false, "Hide Battle Server window.")
	flags.BoolVarP(&values.Force, "force", "f", false, "Force to start more than a single Battle Server per game.")
	flags.BoolVarP(&values.NoErrExisting, "noErrExisting", "r", false, "When 'force' is true and one already exists, exit without error.")
	return
}
