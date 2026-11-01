//go:build !windows

package exec

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"mvdan.cc/sh/v3/syntax"
)

var safeStrings = []string{`&&`}

type arg struct {
	value string
	safe  bool
}

func (a *arg) String() (value string, err error) {
	if a.safe {
		value = a.value
		return
	}
	var quoted string
	if quoted, err = syntax.Quote(a.value, syntax.LangPOSIX); err == nil {
		value = quoted
	}
	return
}

func newUnsafeArg(value string) arg {
	return arg{
		value: value,
		safe:  false,
	}
}

func newSafeArg(value string) arg {
	return arg{
		value: value,
		safe:  true,
	}
}

func stringSliceToArgSlice(args ...string) []arg {
	result := make([]arg, len(args))
	for i, val := range args {
		result[i] = newUnsafeArg(val)
	}
	return result
}

func argSliceToStringSlice(args []arg) []string {
	result := make([]string, len(args))
	for i, val := range args {
		result[i] = val.value
	}
	return result
}

func (options Options) exec() (result *Result) {
	var args []arg
	joinArgsIndex := -1
	if options.ShowWindow {
		args = append(args, terminalArgs()...)
	}
	if options.AsAdmin {
		args = append(args, adminArgs(options.Wait)...)
	}
	if sh := options.Shell || options.ShowWindow; sh {
		args = append(args, shellArgs()...)
		joinArgsIndex = len(args)
		if !options.UseWorkingPath {
			args = append(
				args,
				[]arg{
					newSafeArg("cd"),
					newUnsafeArg(filepath.Dir(options.File)),
					newSafeArg("&&"),
				}...)
		}
	}
	args = append(args, newSafeArg(options.File))
	args = append(args, stringSliceToArgSlice(options.Args...)...)
	if joinArgsIndex != -1 {
		argsQuoted := make([]string, len(args)-joinArgsIndex)
		for i, val := range args[joinArgsIndex:] {
			if slices.Contains(safeStrings, val.value) {
				argsQuoted[i] = val.value
			} else if quoted, err := val.String(); err == nil {
				argsQuoted[i] = quoted
			} else {
				return &Result{
					Err: fmt.Errorf("error quoting argument: %w", err),
				}
			}
		}
		argsReplace := strings.Join(argsQuoted, " ")
		args = args[:joinArgsIndex]
		args = append(args, newSafeArg(argsReplace))
	}
	options.File = args[0].value
	if len(args) > 1 {
		options.Args = argSliceToStringSlice(args[1:])
	}
	return options.standardExec()
}

func shellArgs() []arg {
	return []arg{newSafeArg("sh"), newSafeArg("-c")}
}

func configureCmd(cmd *exec.Cmd, wait bool, _ bool, _ bool) {
	if !wait {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	}
}

func adminArgs(wait bool) []arg {
	if !wait || !common.Interactive() {
		return visualAdminArgs()
	}
	return []arg{newSafeArg("sudo"), newSafeArg("-EH")}
}
