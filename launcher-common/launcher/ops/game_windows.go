package ops

import (
	"errors"

	"github.com/luskaner/ageServer/common/executor/exec"
	"golang.org/x/sys/windows"
)

func adminError(result *exec.Result) bool {
	return errors.Is(result.Err, windows.ERROR_ELEVATION_REQUIRED)
}

// sigKeyArgPath returns the path in the form the client can be given.
//
// The signing key DLL reads the game's command line with a tokenizer that splits
// on spaces and tabs and does not understand quoting, so a path containing a
// space would be cut in half there and the client would end up verifying against
// Microsoft's key without saying so. The same applies to non-ASCII characters:
// the DLL reads the ANSI command line. The short (8.3) form of the path solves
// both, as it names the same file with plain ASCII and no spaces. The bool is
// false when no such form exists, which is the only case where the caller has
// something to warn about.
func sigKeyArgPath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if isClientReadablePath(path) {
		return path, true
	}
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path, false
	}
	short := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(long, &short[0], uint32(len(short)))
	if err != nil || n == 0 || n >= uint32(len(short)) {
		return path, false
	}
	shortPath := windows.UTF16ToString(short[:n])
	if !isClientReadablePath(shortPath) {
		return path, false
	}
	return shortPath, true
}

// isClientReadablePath reports whether a path can appear as one token in the
// command line the signing key DLL reads.
func isClientReadablePath(path string) bool {
	for _, r := range path {
		if r == ' ' || r == '\t' || r > 127 {
			return false
		}
	}
	return true
}
