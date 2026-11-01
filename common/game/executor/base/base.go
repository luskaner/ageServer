package base

import (
	"fmt"

	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
)

type Executor interface {
	Do(args []string, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result)
	GameProcesses() (steamProcess bool, steamMacOsNative bool, xboxProcess bool)
	fmt.Stringer
}
