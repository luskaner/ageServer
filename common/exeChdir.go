package common

import (
	"os"
	"path/filepath"
)

// ProcessInfo abstracts process-level operations for testability.
type ProcessInfo interface {
	Executable() (string, error)
	Chdir(dir string) error
}

type defaultProcessInfo struct{}

func (d *defaultProcessInfo) Executable() (string, error) { return os.Executable() }
func (d *defaultProcessInfo) Chdir(dir string) error      { return os.Chdir(dir) }

var processInfo ProcessInfo = &defaultProcessInfo{}

// SetProcessInfo replaces the process info provider for tests.
func SetProcessInfo(pi ProcessInfo) (restore func()) {
	orig := processInfo
	processInfo = pi
	return func() { processInfo = orig }
}

func ChdirToExe() {
	exePath, err := processInfo.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)
	_ = processInfo.Chdir(exeDir)
}
