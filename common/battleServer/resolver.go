package battleServer

import (
	"path/filepath"

	"github.com/luskaner/ageServer/common/game"
)

const Executable = "BattleServer.exe"

func ResolvePath(gameId string) (ok bool, path string) {
	// AoM has a BattleServer but it's a useless leftover
	if gameId == game.AoM || gameId == game.AoE4 {
		return
	}
	path = Executable
	if gameId == game.AoE2 {
		path = filepath.Join("BattleServer", path)
	}
	ok = true
	return
}
