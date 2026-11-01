package steam

import (
	"runtime"

	"github.com/luskaner/ageServer/common/game"
)

func NewExec(gameId string) (exec *Exec, ok bool) {
	if gameId == game.AoE2 && runtime.GOARCH == "arm64" {
		return newExec(gameId)
	}
	return
}

func (exec Exec) GameProcesses() (steamProcess bool, steamMacOsNative bool, xboxProcess bool) {
	steamMacOsNative = true
	return
}
