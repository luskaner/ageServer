package cmd

import (
	"battle-server-manager/internal"
	"battle-server-manager/internal/cmdUtils"
	"slices"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmd/bsManager"
	"github.com/luskaner/ageServer/common/cmdlog"
)

var (
	parsedGameIdsFnRemove = cmdUtils.ParsedGameIds
	battleServerConfigsFn = battleServer.Configs
	removeFn              = cmdUtils.Remove
)

func runRemove(args []string) (err error, exitCode int) {
	values, flags := bsManager.RemoveFlagSet()
	if err = flags.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	var games mapset.Set[string]
	games, err = parsedGameIdsFnRemove(&values.GameIds)
	if err != nil {
		cmdlog.Fail("%s", err.Error())
		exitCode = internal.ErrGames
		return
	}
	var configs []battleServer.Config
	for g := range games.Iter() {
		cmdlog.Section("Game " + g)
		cmdlog.Step("Removing %s region...", values.Region)
		configs, err = battleServerConfigsFn(g, false, false)
		if err != nil {
			cmdlog.Fault("%s", err)
			continue
		}
		configs = slices.DeleteFunc(configs, func(c battleServer.Config) bool {
			return c.Region != values.Region
		})
		if !removeFn(g, configs, false) {
			cmdlog.Info("No configuration needs it.")
		}
	}
	return
}
