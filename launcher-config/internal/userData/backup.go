package userData

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
)

type Data struct {
	Path string
}

func (d Data) isolatedPath() string {
	if ok, transformedPath := commonUserData.TransformPath(d.Path, commonUserData.TypeActive, commonUserData.TypeServer); ok {
		return transformedPath
	}
	return ""
}

func (d Data) originalPath() string {
	if ok, transformedPath := commonUserData.TransformPath(d.Path, commonUserData.TypeActive, commonUserData.TypeBackup); ok {
		return transformedPath
	}
	return ""
}

func (d Data) switchPaths(backupPath string, currentPath string) (ok bool) {
	commonLogger.Println(commonUi.Detail("Switching %s <-> %s", currentPath, backupPath))
	if _, err := os.Stat(backupPath); err == nil {
		return
	}

	absolutePath := d.Path
	var mode os.FileMode

	if _, err := os.Stat(absolutePath); errors.Is(err, fs.ErrNotExist) {
		oldParent := absolutePath
		newParent := filepath.Dir(oldParent)
		var info os.FileInfo
		for {
			if runtime.GOOS != "windows" {
				if oldParent == newParent {
					return
				}
			} else if newParent == "." {
				return
			}
			info, err = os.Stat(newParent)
			if err == nil {
				mode = info.Mode()
				commonLogger.Println(commonUi.Detail("Creating all path hierarchy: %s", absolutePath))
				if err = os.MkdirAll(absolutePath, mode); err != nil {
					return
				}
				break
			} else if errors.Is(err, fs.ErrNotExist) {
				oldParent = newParent
				newParent = filepath.Dir(newParent)
			} else {
				return
			}
		}
	} else if err != nil {
		return
	}

	commonLogger.Println(commonUi.Detail("Renaming/Moving %s to %s", absolutePath, backupPath))
	if err := os.Rename(absolutePath, backupPath); err != nil {
		return
	}

	var revertMethods []func() bool
	defer func() {
		for _, revertMethod := range slices.Backward(revertMethods) {
			if !revertMethod() {
				break
			}
		}
	}()

	revertMethods = append(revertMethods, func() bool {
		commonLogger.Println(commonUi.Detail("Renaming/Moving %s to %s", backupPath, absolutePath))
		return os.Rename(backupPath, absolutePath) == nil
	})

	if _, err := os.Stat(currentPath); errors.Is(err, fs.ErrNotExist) {
		if mode == 0 {
			var absInfo os.FileInfo
			if absInfo, err = os.Stat(backupPath); err == nil {
				mode = absInfo.Mode()
			} else {
				return
			}
		}
		commonLogger.Println(commonUi.Detail("Making directory %s", currentPath))
		if err = os.Mkdir(currentPath, mode); err != nil {
			return
		}
	} else if err != nil {
		return
	}

	commonLogger.Println(commonUi.Detail("Renaming/Moving %s to %s", currentPath, absolutePath))
	if err := os.Rename(currentPath, absolutePath); err != nil {
		return
	}

	revertMethods = nil
	return true
}

func (d Data) Backup() bool {
	return d.switchPaths(d.originalPath(), d.isolatedPath())
}

func (d Data) Restore() bool {
	return d.switchPaths(d.isolatedPath(), d.originalPath())
}
