//go:build !darwin

package ops

import "github.com/luskaner/ageServer/common/game/executor/base"

func (c *Config) GamePathToGameCertPath(_ base.Executor, path string) string {
	return path
}
