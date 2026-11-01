package launcher_common

import (
	"io"

	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmd/bsManager"
	"github.com/luskaner/ageServer/common/executor/exec"
)

var battleServerExecFn = func(options exec.Options) *exec.Result { return options.Exec() }

func RemoveBattleServerRegion(exe string, gameId string, region string, out io.Writer, optionsFn func(options *exec.Options)) *exec.Result {
	values, flags := bsManager.RemoveFlagSet()
	values.GameIds = []string{gameId}
	values.Region = region
	options := exec.Options{
		File:     exe,
		Wait:     true,
		ExitCode: true,
		Args:     cmd.FlagSetToArgs(flags, true),
	}
	if optionsFn != nil {
		optionsFn(&options)
	}
	if out != nil {
		options.Stdout = out
		options.Stderr = out
	}
	return battleServerExecFn(options)
}
