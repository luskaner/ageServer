package executables

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Server

const Server = "server"
const ServerGenCert = "genCert"

// Launcher

const Launcher = "launcher"
const LauncherAgent = "agent"
const LauncherConfig = "config"
const ConfigHelper = "config-helper"
const LauncherConfigAdmin = "config-admin"
const LauncherConfigAdminAgent = "config-admin-agent"

// Battle Server Manager

const BattleServerManager = "battle-server-manager"

var directories = []string{
	fmt.Sprintf(`%c`, filepath.Separator),
	fmt.Sprintf(`%c..%c`, filepath.Separator, filepath.Separator),
	fmt.Sprintf(`%c..%c..%c`, filepath.Separator, filepath.Separator, filepath.Separator),
}

func NativeFileName(bin bool, executable string) string {
	return FileName(bin, executable, nil)
}

func FileName(bin bool, executable string, transfileName func(name string) string) string {
	if transfileName == nil {
		transfileName = fileName
	}
	filename := transfileName(executable)
	if !bin {
		filename = filepath.Join("bin", filename)
	}
	return filename
}

func ArchFileName(bin bool, executable string, transfileName func(name string) string) string {
	return FileName(bin, fmt.Sprintf("%s_%s", executable, runtime.GOARCH), transfileName)
}

func WindowsFileName(name string) string {
	return name + ".exe"
}

func UnixFileName(name string) string {
	return name
}

func BaseNameNoExt(fileName string) string {
	extension := filepath.Ext(fileName)
	return strings.TrimSuffix(fileName, extension)
}

var (
	osExecutableFn = os.Executable
	osStatFn       = os.Stat
)

func FindPath(executableName string) string {
	ex, err := osExecutableFn()
	if err != nil {
		return ""
	}
	exeDir := BaseNameNoExt(executableName)
	exePath := filepath.Dir(ex)
	var f os.FileInfo
	for _, dir := range directories {
		executablePath := filepath.Join(exePath, dir, exeDir, executableName)
		if f, err = osStatFn(executablePath); err == nil && !f.IsDir() {
			executablePath, _ = filepath.Abs(executablePath)
			return executablePath
		}
	}
	return ""
}
