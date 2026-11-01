package launcher_common

import (
	"os"
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
)

func storeCommand(t *testing.T, cmd []string) {
	t.Helper()
	if err := RevertCommandStore.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := RevertCommandStore.Store(cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RevertCommandStore.Delete() })
}

// Regression: RunRevertCommand deleted the stored command even when execution
// FAILED, losing the elevated action forever. It must only clear the store on
// success, mirroring ConfigRevert.
func TestRunRevertCommandKeepsStoreOnFailure(t *testing.T) {
	storeCommand(t, []string{"fake-exe", "--do-stuff"})

	var received exec.Options
	r := newReverter(deps{exec: func(options exec.Options) *exec.Result {
		received = options
		return &exec.Result{Err: os.ErrPermission, ExitCode: 1}
	}})

	err := r.RunRevertCommand(nil, nil)
	if err == nil {
		t.Fatal("expected the failure error to propagate")
	}
	if received.File != "fake-exe" || len(received.Args) != 1 || received.Args[0] != "--do-stuff" {
		t.Fatalf("options not forwarded: %+v", received)
	}

	err, flags := RevertCommandStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) == 0 {
		t.Fatal("store was deleted on failure; failed command is lost and can never be retried")
	}
}

func TestRunRevertCommandClearsStoreOnSuccess(t *testing.T) {
	storeCommand(t, []string{"fake-exe", "--ok"})

	r := newReverter(deps{exec: func(exec.Options) *exec.Result {
		return &exec.Result{}
	}})

	if err := r.RunRevertCommand(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err, flags := RevertCommandStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 0 {
		t.Fatalf("store must be cleared on success, still has %v", flags)
	}
}

func TestRunRevertCommandNoopWithoutStore(t *testing.T) {
	if err := RevertCommandStore.Delete(); err != nil {
		t.Fatal(err)
	}
	execRan := false
	r := newReverter(deps{exec: func(exec.Options) *exec.Result {
		execRan = true
		return &exec.Result{}
	}})

	if err := r.RunRevertCommand(nil, nil); err != nil {
		t.Fatal(err)
	}
	if execRan {
		t.Fatal("must not execute anything without a stored command")
	}
}

func TestRunRevertCommandOptionsFnMutates(t *testing.T) {
	storeCommand(t, []string{"fake-exe"})
	var seenInjected bool
	r := newReverter(deps{exec: func(opts exec.Options) *exec.Result {
		if len(opts.Args) == 1 && opts.Args[0] == "injected" {
			seenInjected = true
		}
		return &exec.Result{}
	}})

	err := r.RunRevertCommand(nil, func(o *exec.Options) {
		o.Args = []string{"injected"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seenInjected {
		t.Fatal("optionsFn pointer mutation was not applied to exec options")
	}
}

func TestRunRevertCommandOutputRedirection(t *testing.T) {
	storeCommand(t, []string{"fake-exe"})
	var captured *exec.Options
	r := newReverter(deps{exec: func(opts exec.Options) *exec.Result {
		captured = &opts
		return &exec.Result{}
	}})

	var buf os.File
	if err := r.RunRevertCommand(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if captured.Stdout != &buf || captured.Stderr != &buf {
		t.Fatal("out should be assigned to Stdout/Stderr")
	}
}

func TestRunRevertCommandSingleArgNoArgs(t *testing.T) {
	storeCommand(t, []string{"only-exe"})
	var received exec.Options
	r := newReverter(deps{exec: func(o exec.Options) *exec.Result {
		received = o
		return &exec.Result{}
	}})

	if err := r.RunRevertCommand(nil, nil); err != nil {
		t.Fatal(err)
	}
	if received.File != "only-exe" {
		t.Fatalf("File = %q, want only-exe", received.File)
	}
	if len(received.Args) != 0 {
		t.Fatalf("Args should be empty for single cmd, got %v", received.Args)
	}
}
