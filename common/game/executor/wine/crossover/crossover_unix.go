//go:build !windows

package crossover

import (
	"os"

	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/game/wine/steam/crossover"
)

func binPath() string {
	return game.FirstExistingFile(files, nil, func(f os.FileInfo) bool {
		return !f.IsDir()
	})
}

type Exec struct {
	prefix  string
	binPath string
	gameId  string
}

func (exec Exec) DoCustom(args []string, optionsFn func(options *commonExecutor.Options)) (result *commonExecutor.Result) {
	options := commonExecutor.Options{
		File: exec.binPath,
		Args: []string{"--bottle", exec.prefix},
		Pid:  true,
	}
	options.Args = append(options.Args, args...)
	if optionsFn != nil {
		optionsFn(&options)
	}
	result = options.Exec()
	return
}

func NewExec(gameId string) *Exec {
	if prefix := crossover.Prefix(gameId); prefix != "" {
		if binary := binPath(); binary != "" {
			return &Exec{prefix, binary, gameId}
		}
	}
	return nil
}
