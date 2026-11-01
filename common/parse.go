package common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"mvdan.cc/sh/v3/shell"
)

var reWinToLinVar *regexp.Regexp

// osStatFn is injectable for tests.
var parseStatFn = os.Stat

func ParseCommandArgsFromSlice(value []string, values map[string]string, separateFields bool) (args []string, err error) {
	cmdArgs := strings.Join(value, " ")
	for key, val := range values {
		cmdArgs = strings.ReplaceAll(cmdArgs, fmt.Sprintf(`{%s}`, key), val)
	}
	if runtime.GOOS == "windows" {
		if reWinToLinVar == nil {
			reWinToLinVar = regexp.MustCompile(`%(\w+)%`)
		}
		cmdArgs = reWinToLinVar.ReplaceAllString(cmdArgs, `$$$1`)
	}
	if separateFields {
		args, err = shell.Fields(cmdArgs, nil)
	} else {
		args = []string{cmdArgs}
	}
	return
}

func ParsePath(value []string, values map[string]string) (file os.FileInfo, path string, err error) {
	var args []string
	args, err = ParseCommandArgsFromSlice(value, values, false)
	if err != nil {
		return
	}
	if len(args) != 1 {
		err = fmt.Errorf("invalid path")
		return
	}
	path, err = filepath.Abs(args[0])
	if err != nil {
		return
	}
	file, err = parseStatFn(path)
	return
}

func EnhancedViperStringToStringSlice(value string) []string {
	return []string{value}
}
