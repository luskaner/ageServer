package admin

import (
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/spf13/pflag"
)

type RevertValues struct {
	*config.RevertBaseValues
	*commonCmd.LogRootValues
}

func newRevertValues() RevertValues {
	return RevertValues{
		RevertBaseValues: &config.RevertBaseValues{},
		LogRootValues:    &commonCmd.LogRootValues{},
	}
}

func RevertFlagSet() (values *RevertValues, flags *pflag.FlagSet) {
	values = new(newRevertValues())
	flags = pflag.NewFlagSet("revert", pflag.ContinueOnError)
	values.RevertBaseValues = config.InitBaseRevert(flags)
	commonCmd.LogRootCommand(flags, values.LogRootRef())
	return
}
