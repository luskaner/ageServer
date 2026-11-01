package ops

import (
	"os"

	"github.com/luskaner/ageServer/common/game/executor/base"
)

func (c *Config) GamePathToGameCertPath(executer base.Executor, path string) string {
	if c.NativeMacOsGame(executer, false) {
		return os.ExpandEnv("$HOME") + `/Library/Application Support/Steam/steamapps/common/AoE2DE/AgeOfEmpires2Data`
	}
	return path
}
