package ops

import (
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/executor/base"
)

func (c *Config) IsolationPath(_ base.Executor) (path string) {
	return game.UserProfilePath(c.gameId)
}
