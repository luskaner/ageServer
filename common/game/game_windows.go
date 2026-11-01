package game

import (
	"os"

	"golang.org/x/sys/windows"
)

var knownFolderPathFn = func(folderID *windows.KNOWNFOLDERID, flags uint32) (string, error) {
	return windows.KnownFolderPath(folderID, flags)
}

func UserProfilePath(gameId string) string {
	if gameId == AoE4 {
		if path, err := knownFolderPathFn(windows.FOLDERID_Documents, 0); err == nil {
			return path
		}
		return ""
	}
	return os.Getenv("USERPROFILE")
}
