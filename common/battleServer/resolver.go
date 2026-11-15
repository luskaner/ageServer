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

// ResolvePathPriority returns the list of game IDs to try when resolving the battle server path,
// in order of preference. This allows using battle servers from other games as fallbacks.
func ResolvePathPriority(gameId string) []string {
	switch gameId {
	case game.AoE1:
		// For AoE1: try AoE1, then AoE2, then AoE3
		return []string{game.AoE1, game.AoE2, game.AoE3}
	case game.AoE3:
		// For AoE3: try AoE2, then AoE3, then AoE1
		return []string{game.AoE2, game.AoE3, game.AoE1}
	default:
		// For other games (AoE2, AoE4, AoM): just use their own
		return []string{gameId}
	}
}
