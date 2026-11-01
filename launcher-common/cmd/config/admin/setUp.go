package admin

import (
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/launcher-common/cmd/config"
	"github.com/spf13/pflag"
)

type SetupValues struct {
	*config.SetupBaseValues
	*commonCmd.GameIdValues
	*commonCmd.LogRootValues
}

func newSetupValues() SetupValues {
	return SetupValues{
		SetupBaseValues: &config.SetupBaseValues{},
		GameIdValues:    &commonCmd.GameIdValues{},
		LogRootValues:   &commonCmd.LogRootValues{},
	}
}

func SetupFlagSet() (values *SetupValues, flags *pflag.FlagSet) {
	values = new(newSetupValues())
	flags = pflag.NewFlagSet("setup", pflag.ContinueOnError)
	values.SetupBaseValues = config.InitSetUp(flags)
	commonCmd.LogRootCommand(flags, values.LogRootRef())
	commonCmd.GameVarCommand(flags, values.GameIdRef())
	return
}
