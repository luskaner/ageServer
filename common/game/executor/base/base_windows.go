package base

import (
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
)

func StartUri(uri string, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result) {
	options := commonExecutor.Options{File: uri, Shell: true, SpecialFile: true, ShowWindow: true}
	optionsFn(options)
	result = options.Exec()
	return
}
