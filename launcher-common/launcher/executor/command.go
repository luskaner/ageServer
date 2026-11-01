package executor

import (
	"io"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/certStore"
	"github.com/luskaner/ageServer/common/executor/exec"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
)

func RunRevertCommand(out io.Writer, optionsFn func(options *exec.Options)) (err error) {
	err = launcherCommon.RunRevertCommand(out, optionsFn)
	certStore.ReloadSystemCertificates()
	common.ClearDNSCache()
	return
}
