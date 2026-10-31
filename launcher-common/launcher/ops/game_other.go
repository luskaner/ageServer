//go:build !windows

package ops

import (
	"os"

	"github.com/luskaner/ageServer/common/executor/exec"
)

func adminError(result *exec.Result) bool {
	return os.IsPermission(result.Err)
}

// sigKeyArgPath returns the path in the form the client can be given.
//
// The constraint that forces a rewrite on Windows is the signing key DLL's
// command line tokenizer, and that DLL only runs inside the game; a POSIX path
// given to a Windows launcher is a separate question the launcher companion
// already answers for the certificate file too. Nothing to rewrite here.
func sigKeyArgPath(path string) (string, bool) {
	return path, path != ""
}
