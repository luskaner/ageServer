package ipc

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathPerPlatform(t *testing.T) {
	p := Path()
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(p, `\\.\pipe\`) || !strings.HasSuffix(p, name) {
			t.Fatalf("windows pipe path = %q", p)
		}
		return
	}
	want := filepath.Join(os.TempDir(), name)
	if p != want {
		t.Fatalf("unix path = %q, want %q", p, want)
	}
}

func TestCommandConstantsDistinct(t *testing.T) {
	if Revert == Setup || Setup == Exit || Revert == Exit {
		t.Fatal("IPC command constants must be distinct")
	}
}
