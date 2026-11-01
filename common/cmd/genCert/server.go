package genCert

import (
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type Values struct {
	Replace          bool
	IgnoreIfExisting bool
}

func SingleFlagSet(version string, runFn func(*pflag.FlagSet) (err error, exitCode int)) (values *Values, singleFs *cmd.SingleFlagSet) {
	singleFs = cmd.NewSingleFlagSet(runFn, version)
	flags := singleFs.Fs()
	values = &Values{}
	flags.BoolVarP(&values.Replace, "replace", "r", false, "Overwrite existing certificate pairs.")
	flags.BoolVarP(&values.IgnoreIfExisting, "ignoreIfExisting", "i", false, "Do not error out if the certificate pairs already exist.")
	return
}
