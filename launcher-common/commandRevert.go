package launcher_common

import (
	"io"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
)

var RevertCommandStore = NewArgsStore(filepath.Join(os.TempDir(), common.Name+"_command_revert.txt"))

func (r *Reverter) RunRevertCommand(out io.Writer, optionsFn func(options *exec.Options)) (err error) {
	var args []string
	var cmd []string
	err, cmd = RevertCommandStore.Load()
	if err != nil || len(cmd) == 0 {
		return
	}
	if len(cmd) > 1 {
		args = cmd[1:]
	}
	options := exec.Options{
		File:           cmd[0],
		Wait:           true,
		SpecialFile:    true,
		UseWorkingPath: true,
		Args:           args,
	}
	if optionsFn != nil {
		optionsFn(&options)
	}
	if out != nil {
		options.Stdout = out
		options.Stderr = out
	}
	result := r.deps.exec(options)
	err = result.Err
	// Keep the stored command when execution failed so it can be retried,
	// mirroring ConfigRevert's store handling.
	if result.Success() {
		_ = RevertCommandStore.Delete()
	}
	return
}

func RunRevertCommand(out io.Writer, optionsFn func(options *exec.Options)) (err error) {
	return Default.RunRevertCommand(out, optionsFn)
}
