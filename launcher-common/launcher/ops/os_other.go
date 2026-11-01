//go:build !darwin

package ops

import "github.com/luskaner/ageServer/common/game/executor/base"

func (c *Config) NativeMacOsGame(_ base.Executor, _ bool) bool {
	return false
}
