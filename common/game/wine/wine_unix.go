//go:build !windows

package wine

import (
	"path/filepath"

	"github.com/luskaner/ageServer/common/game"
)

var mainDirs = []string{
	"$WINEPREFIX",
}

var altDirs = []string{
	// TODO: Add system wide installation for macOS
	"$HOME/.wine",
}

func Prefix() string {
	prefix := game.FirstExistingDir(mainDirs, nil)
	if prefix != "" {
		return prefix
	}
	return game.FirstExistingDir(altDirs, func(s string) string {
		return filepath.Join(s)
	})
}
func UserProfile(user string) string {
	return filepath.Join("dosdevices", "c:/", "users", user)
}
