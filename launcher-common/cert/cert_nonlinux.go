//go:build !linux

package cert

import "github.com/luskaner/ageServer/common/executor/exec"

func FlushCerts() (result *exec.Result) {
	return exec.ResultSuccess
}
