package bsManager

import (
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type RemoveValues struct {
	GameIds []string
	Region  string
}

func RemoveFlagSet() (values *RemoveValues, flags *pflag.FlagSet) {
	values = new(RemoveValues)
	flags = pflag.NewFlagSet("remove", pflag.ContinueOnError)
	flags.StringVarP(&values.Region, "region", "r", "", "Region of the battle server")
	cmd.GamesVarCommand(flags, &values.GameIds)
	return
}
