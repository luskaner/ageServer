package config

import (
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/spf13/pflag"
)

type FlushCacheValues struct {
	*RevertMinimalValues
	*commonCmd.LogRootValues
}

func newFlushCacheValues() FlushCacheValues {
	return FlushCacheValues{
		RevertMinimalValues: NewRevertMinimalValues(),
		LogRootValues:       &commonCmd.LogRootValues{},
	}
}

func setFlushCacheFlags(values *FlushCacheValues, flags *pflag.FlagSet) {
	flags.BoolVarP(
		values.IPsRef(),
		"flushIpCache",
		"i",
		false,
		"Flush the IP mappings local DNS cache",
	)
	flags.BoolVarP(
		values.CertsRef(),
		"flushCertsCache",
		"c",
		false,
		"Flush the certificate cache from the local machine's trusted root store",
	)
	commonCmd.LogRootCommand(flags, values.LogRootRef())
}

func FlushCacheFlagSet() (values *FlushCacheValues, flags *pflag.FlagSet) {
	values = new(newFlushCacheValues())
	flags = pflag.NewFlagSet("flushCache", pflag.ContinueOnError)
	setFlushCacheFlags(values, flags)
	return
}

func FlushCacheSingleFlagSet(version string, runFn func(*pflag.FlagSet) (err error, exitCode int)) (values *FlushCacheValues, singleFs *commonCmd.SingleFlagSet) {
	values = new(newFlushCacheValues())
	singleFs = commonCmd.NewSingleFlagSet(runFn, version)
	setFlushCacheFlags(values, singleFs.Fs())
	return
}
