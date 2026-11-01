package steam

import (
	"os"

	"github.com/luskaner/ageServer/common/game"
)

const applicationSupport = `/Library/Application Support/`

var dirs = []string{
	// Official
	"$HOME" + applicationSupport + `Steam`,
}

func ConfigPath() string {
	return game.FirstExistingDir(dirs, func(s string) string {
		return s
	})
}

func ConfigPathAlt() (path string) {
	// No known alternatives
	return
}

func UserProfilePath(gameId string) string {
	if gameId != game.AoE2 {
		return ""
	}
	return os.ExpandEnv("$HOME") + applicationSupport + "Feral Interactive/Age Of Empires II/VFS/User"
}
