package cmd

import (
	"battle-server-manager/internal/cmdUtils"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/spf13/pflag"
)

var removeAllFnClean = cmdUtils.RemoveAll

func runClean(args []string) (err error, exitCode int) {
	fs := pflag.NewFlagSet("clean", pflag.ContinueOnError)
	cmd.GamesVarCommand(fs, &cmdUtils.GameIds)
	if err = fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	cmdlog.Step("Cleaning up...")
	return removeAllFnClean(true)
}
