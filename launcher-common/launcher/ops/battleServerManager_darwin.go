package ops

import (
	"github.com/luskaner/ageServer/common/game/executor/base"
)

func (c *Config) BattleServerRequired(executer base.Executor) bool {
	return c.gameRequiresBattleServer() || c.NativeMacOsGame(executer, true)
}
