package steam

import (
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/executor/base"
	"github.com/luskaner/ageServer/common/game/steam"
)

func newExec(gameId string) (exec *Exec, ok bool) {
	var game *steam.Game
	if game, ok = steam.NewGame(gameId); ok {
		exec, ok = NewExecFromGame(game)
	}
	return
}

type Exec struct {
	*steam.Game
}

func (exec Exec) String() string {
	return "Steam"
}

func (exec Exec) Do(_ []string, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result) {
	return base.StartUri(exec.OpenUri(), optionsFn)
}

func NewExecFromGame(game *steam.Game) (exec *Exec, ok bool) {
	if game != nil {
		exec = &Exec{game}
		ok = true
	}
	return
}
