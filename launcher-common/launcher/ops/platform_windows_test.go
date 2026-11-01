package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
	"golang.org/x/sys/windows"
)

// Split out of platform_test.go: ERROR_ELEVATION_REQUIRED is a Windows
// constant. Moved verbatim; only the location changed.
func TestAdminErrorWithElevationRequired(t *testing.T) {
	result := &exec.Result{Err: windows.ERROR_ELEVATION_REQUIRED}
	if !adminError(result) {
		t.Error("expected true for ERROR_ELEVATION_REQUIRED")
	}
}

// The signing key DLL splits the game's command line on spaces and does not strip
// quotes, so a temporary folder with a space in it would silently break signing.
// The short form of the path names the same file without one.
func TestSigKeyArgPathUsesTheShortFormWhenThePathHasSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a folder with spaces")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sigkey.der")
	if err := os.WriteFile(path, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}

	got, ok := sigKeyArgPath(path)
	if !ok {
		t.Skip("this volume creates no 8.3 names, so there is no usable form to test")
	}
	if strings.ContainsAny(got, " \t") {
		t.Fatalf("arg path %q still cannot be one command line token", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("arg path %q does not name the key file: %v", got, err)
	}
}

// A path that is already one token is the one the user sees in the logs, so it
// must not be rewritten for no reason.
func TestSigKeyArgPathLeavesAPlainPathAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sigkey.der")
	got, ok := sigKeyArgPath(path)
	if !ok {
		t.Fatalf("plain ASCII path %q reported as unusable", path)
	}
	if got != path {
		t.Errorf("path was rewritten to %q", got)
	}
}
