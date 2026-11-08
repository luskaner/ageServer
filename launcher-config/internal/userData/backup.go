package userData

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

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

// renameWithRetry renames from to to, retrying a few times with a growing
// backoff. A directory can be transiently held right after the game exits (the
// indexer or the antivirus still scanning it) and a single attempt turns that
// into a failed revert. A persistent lock, such as a process that keeps a
// handle open on the folder, still fails, but the caller now logs the reason.
func renameWithRetry(from string, to string) error {
	const attempts = 5
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		if attempt < attempts-1 {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	return err
}

// logSwitchFailure reports why a path could not be switched. Without it the log
// shows the "Renaming/Moving" line and then nothing, which reads as a hang and
// hides the actual cause: a locked folder, a missing parent or a denied rename.
func logSwitchFailure(action string, path string, err error) {
	commonLogger.Println(commonUi.Fail("Failed to %s %s", action, path))
	commonLogger.Println(commonUi.Detail("%s", err))
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
					logSwitchFailure("find an existing parent of", absolutePath, fs.ErrNotExist)
					return
				}
			} else if newParent == "." {
				logSwitchFailure("find an existing parent of", absolutePath, fs.ErrNotExist)
				return
			}
			info, err = os.Stat(newParent)
			if err == nil {
				mode = info.Mode()
				commonLogger.Println(commonUi.Detail("Creating all path hierarchy: %s", absolutePath))
				if err = os.MkdirAll(absolutePath, mode); err != nil {
					logSwitchFailure("create", absolutePath, err)
					return
				}
				break
			} else if errors.Is(err, fs.ErrNotExist) {
				oldParent = newParent
				newParent = filepath.Dir(newParent)
			} else {
				logSwitchFailure("stat", newParent, err)
				return
			}
		}
	} else if err != nil {
		logSwitchFailure("stat", absolutePath, err)
		return
	}

	commonLogger.Println(commonUi.Detail("Renaming/Moving %s to %s", absolutePath, backupPath))
	if err := renameWithRetry(absolutePath, backupPath); err != nil {
		logSwitchFailure("rename", absolutePath, err)
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
		if err := renameWithRetry(backupPath, absolutePath); err != nil {
			logSwitchFailure("roll back rename", backupPath, err)
			return false
		}
		return true
	})

	if _, err := os.Stat(currentPath); errors.Is(err, fs.ErrNotExist) {
		if mode == 0 {
			var absInfo os.FileInfo
			if absInfo, err = os.Stat(backupPath); err == nil {
				mode = absInfo.Mode()
			} else {
				logSwitchFailure("stat", backupPath, err)
				return
			}
		}
		commonLogger.Println(commonUi.Detail("Making directory %s", currentPath))
		if err = os.Mkdir(currentPath, mode); err != nil {
			logSwitchFailure("create", currentPath, err)
			return
		}
	} else if err != nil {
		logSwitchFailure("stat", currentPath, err)
		return
	}

	commonLogger.Println(commonUi.Detail("Renaming/Moving %s to %s", currentPath, absolutePath))
	if err := renameWithRetry(currentPath, absolutePath); err != nil {
		logSwitchFailure("rename", currentPath, err)
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
