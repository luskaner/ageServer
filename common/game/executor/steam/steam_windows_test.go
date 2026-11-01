package steam

import (
	"testing"

	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game/steam"
	"golang.org/x/sys/windows"
)

// Split out of steam_test.go: TestDo needs the ShellExecuteExW seams, which
// only exist on Windows. Verbatim from steam_test.go; only the location moved.
func TestDo(t *testing.T) {
	restore := commonExecutor.SetShellExecuteExCallFn(func(_ *commonExecutor.SHELLEXECUTEINFO) (uintptr, uintptr, error) { return 1, 0, nil })
	defer restore()
	commonExecutor.SetGetProcessIdFn(func(_ windows.Handle) (uint32, error) { return 4321, nil })
	g := &steam.Game{}
	e, ok := NewExecFromGame(g)
	if !ok {
		t.Fatal("NewExecFromGame should succeed")
	}
	r := e.Do(nil, func(commonExecutor.Options) {})
	if r == nil {
		t.Fatal("result should not be nil")
	}
}
