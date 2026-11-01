package steam

import (
	"path/filepath"
	"strings"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/wine"
)

const suffixDir = ".steam/steam"

var dirs = []string{
	// Official
	"$HOME/{suffixDir}",
	// Alternative official
	"$HOME/.local/share/Steam",
	// Snap
	"$HOME/snap/steam/common/{suffixDir}",
	// Flatpak
	"$HOME/.var/app/com.valvesoftware.Steam/{suffixDir}",
	// Less common from https://github.com/lutris/lutris/blob/master/lutris/util/steam/config.py
	"$HOME/.steam/debian-installation",
	"$HOME/.steam",
	"$HOME/.local/share/steam",
	"$HOME/snap/steam/common/.local/share/Steam",
	"$HOME/.var/app/com.valvesoftware.Steam/data/Steam",
	"/usr/share/steam",
	"/usr/local/share/steam",
}

func ConfigPath() string {
	return game.FirstExistingDir(dirs, func(s string) string {
		return strings.ReplaceAll(s, "{suffixDir}", suffixDir)
	})
}

func ConfigPathAlt() (path string) {
	// No known alternatives
	return
}

func UserProfilePath(gameId string) string {
	suffix := ConfigPath()
	if suffix == "" {
		return ""
	}
	return filepath.Join(suffix, "steamapps", "compatdata", appId(gameId), "pfx", wine.UserProfile("steamuser"))
}
