//go:build !windows

package steam

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/executor/wine"
	"github.com/luskaner/ageServer/common/game/steam"
)

type Game struct {
	*steam.Game
}

func NewGame(gameId string, executor wine.CustomExec) (game *steam.Game, ok bool) {
	return steam.NewCustomGame(
		gameId,
		func() string {
			return run(executor, false)
		},
		func() string {
			return run(executor, true)
		},
		func(s string) string {
			return OutputLauncherConfigHelper(executor, "windowsToUnixPath", []string{s})
		},
	)
}

func run(executor wine.CustomExec, alt bool) (path string) {
	return OutputLauncherConfigHelper(executor, "configPath", []string{strconv.FormatBool(alt)})
}

func OutputLauncherConfigHelper(executor wine.CustomExec, method string, args []string) (output string) {
	absExePath, err := filepath.Abs(executables.ArchFileName(false, executables.ConfigHelper, executables.WindowsFileName))
	if err != nil {
		return
	}
	var sb strings.Builder
	fullArgs := []string{
		absExePath,
		method,
	}
	fullArgs = append(fullArgs, args...)
	result := executor.DoCustom(
		fullArgs,
		func(options *exec.Options) {
			options.UseWorkingPath = true
			options.Pid = false
			options.ExitCode = true
			options.Wait = true
			options.Stdout = &sb
		},
	)
	if result.Success() {
		output = sb.String()
	}
	return
}
