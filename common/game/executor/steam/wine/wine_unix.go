//go:build !windows

package wine

import (
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/executor/steam"
	"github.com/luskaner/ageServer/common/game/executor/wine"
	wineSteam "github.com/luskaner/ageServer/common/game/wine/steam"
)

type Exec struct {
	*steam.Exec
	exec *wine.Exec
}

func (exec Exec) Do(args []string, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result) {
	finalArgs := []string{"start", exec.OpenUri()}
	finalArgs = append(finalArgs, args...)
	return exec.exec.DoCustom(finalArgs, func(options *commonExecutor.Options) {
		optionsFn(*options)
	})
}

func (exec Exec) DoCustom(args []string, optionsFn func(options *commonExecutor.Options)) (result *commonExecutor.Result) {
	return exec.exec.DoCustom(args, optionsFn)
}

func (exec Exec) String() string {
	return exec.Exec.String() + " with Wine"
}

func NewExec(gameId string) (exec *Exec, ok bool) {
	if executor, localOk := wine.NewExec(); localOk {
		if steamGame, localOk := wineSteam.NewGame(gameId, executor); localOk {
			var steamExec *steam.Exec
			if steamExec, ok = steam.NewExecFromGame(steamGame); ok {
				exec = &Exec{
					steamExec,
					executor,
				}
			}
		}
	}
	return
}
