//go:build !windows

package wine

import (
	"github.com/hairyhenderson/go-which"
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/wine"
)

type CustomExec interface {
	DoCustom(args []string, optionsFn func(options *commonExecutor.Options)) (result *commonExecutor.Result)
}

type Exec struct{}

func NewExec() (exec *Exec, ok bool) {
	if wine.Prefix() != "" {
		exec = &Exec{}
		ok = true
	} else if ok = which.Found("wine"); ok {
		exec = &Exec{}
	}
	return
}

func (exec Exec) DoCustom(args []string, optionsFn func(options *commonExecutor.Options)) (result *commonExecutor.Result) {
	options := commonExecutor.Options{
		File:        "wine",
		SpecialFile: true,
		Args:        args,
		Pid:         true,
	}
	if optionsFn != nil {
		optionsFn(&options)
	}
	result = options.Exec()
	return
}
