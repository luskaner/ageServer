//go:build !windows

package ipc

import (
	"os"
	"testing"

	lipc "github.com/luskaner/ageServer/launcher-common/ipc"
)

// Regression: RevertServer removed path.Join() (empty string) instead of the
// actual socket path, leaking the stale unix socket file between runs.
func TestRevertServerRemovesSocketPath(t *testing.T) {
	socketPath := lipc.Path()
	t.Cleanup(func() { _ = os.Remove(socketPath) })

	if err := os.WriteFile(socketPath, []byte("stale"), 0666); err != nil {
		t.Fatal(err)
	}

	RevertServer()

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatal("socket path was not removed by RevertServer")
	}
}
