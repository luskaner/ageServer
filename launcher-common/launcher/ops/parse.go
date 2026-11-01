package ops

import (
	"github.com/luskaner/ageServer/common"
)

func ParseCommandArgs(cmdSlice []string, values map[string]string) (args []string, err error) {
	return common.ParseCommandArgsFromSlice(cmdSlice, values, true)
}
