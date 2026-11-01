package xbox

import (
	"fmt"

	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/appx"
)

type Exec struct {
	*appx.Game
}

func (exec Exec) Do(_ []string, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result) {
	options := commonExecutor.Options{
		File:        fmt.Sprintf(`shell:appsfolder\%s!App`, exec.FamilyName()),
		Shell:       true,
		SpecialFile: true,
		ShowWindow:  true,
	}
	optionsFn(options)
	result = options.Exec()
	return
}

func (exec Exec) GamePath() string {
	return exec.Path()
}

func (exec Exec) GameProcesses() (steamProcess bool, steamMacOsNative bool, xboxProcess bool) {
	xboxProcess = true
	return
}

func (exec Exec) String() string {
	return "Xbox"
}

func NewExec(gameId string) (exec *Exec, ok bool) {
	if gameId != game.AoM {
		var g *appx.Game
		if g, ok = appx.NewGame(gameId); ok {
			exec = &Exec{g}
			ok = true
		}
	}
	return
}
