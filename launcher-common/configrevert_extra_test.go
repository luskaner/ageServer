package launcher_common

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
)

func tempRevertStore(t *testing.T) {
	t.Helper()
	orig := RevertConfigStore
	tmp := NewArgsStore(filepath.Join(t.TempDir(), "revert.txt"))
	RevertConfigStore = tmp
	t.Cleanup(func() { RevertConfigStore = orig })
}

func TestConfigRevert_NoStoreIsSuccess(t *testing.T) {
	tempRevertStore(t)
	called := false
	mock := func(flags []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		called = true
		return &exec.Result{}
	}
	r := newReverter(deps{})
	ok := r.ConfigRevert("age2", "", false, nil, nil, mock)
	if !ok {
		t.Error("ConfigRevert with empty store should return success true")
	}
	if called {
		t.Error("runRevert should not be called when store is empty")
	}
}

func TestConfigRevert_SuccessDeletesStore(t *testing.T) {
	tempRevertStore(t)
	opts := NewConfigRevertFlagOptions()
	opts.IPs = true
	opts.HostFilePath = "/tmp/hosts"
	flags := opts.Flags()
	if err := RevertConfigStore.Store(flags); err != nil {
		t.Fatal(err)
	}

	mock := func(f []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		if len(f) == 0 {
			t.Error("flags should not be empty")
		}
		return &exec.Result{}
	}
	r := newReverter(deps{isAdmin: func() bool { return true }, agentRunning: func(bool) bool { return false }})
	ok := r.ConfigRevert("age2", "", false, nil, nil, mock)
	if !ok {
		t.Error("expected success true when mock succeeds")
	}
	err, stored := RevertConfigStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("store should be deleted after success, still has %v", stored)
	}
}

func TestConfigRevert_FailureKeepsStore(t *testing.T) {
	tempRevertStore(t)
	opts := NewConfigRevertFlagOptions()
	opts.IPs = true
	opts.HostFilePath = "/tmp/hosts"
	flags := opts.Flags()
	if err := RevertConfigStore.Store(flags); err != nil {
		t.Fatal(err)
	}

	mock := func(f []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		return &exec.Result{Err: os.ErrPermission, ExitCode: 1}
	}
	r := newReverter(deps{isAdmin: func() bool { return true }, agentRunning: func(bool) bool { return false }})
	ok := r.ConfigRevert("age2", "", false, nil, nil, mock)
	if ok {
		t.Error("expected success false when revert fails")
	}
	err, stored := RevertConfigStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) == 0 {
		t.Error("store should be kept after failure")
	}
}

func TestConfigRevert_ParseErrorFallbackAllGames(t *testing.T) {
	tempRevertStore(t)
	if err := RevertConfigStore.Store([]string{"--invalid-flag-xyz"}); err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	mock := func(f []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		calls = append(calls, f)
		return &exec.Result{}
	}
	r := newReverter(deps{isAdmin: func() bool { return true }, agentRunning: func(bool) bool { return false }})
	ok := r.ConfigRevert("", "/tmp/logs", false, nil, nil, mock)
	if !ok {
		t.Error("expected success")
	}
	wantCalls := len(game.SupportedGames.ToSlice())
	if len(calls) != wantCalls {
		t.Fatalf("expected %d calls for all games fallback, got %d: %v", wantCalls, len(calls), calls)
	}
	for _, f := range calls {
		found := false
		for _, a := range f {
			if a == "--all" {
				found = true
			}
		}
		if !found {
			t.Errorf("fallback flags should contain --all, got %v", f)
		}
	}
}

func TestConfigRevert_HeadlessRequiresAdminSkips(t *testing.T) {
	tempRevertStore(t)
	opts := NewConfigRevertFlagOptions()
	opts.Certs = true // no CertFilePath => requires admin
	flags := opts.Flags()
	if err := RevertConfigStore.Store(flags); err != nil {
		t.Fatal(err)
	}

	called := false
	mock := func(f []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		called = true
		return &exec.Result{}
	}
	r := newReverter(deps{isAdmin: func() bool { return false }, agentRunning: func(bool) bool { return false }})
	ok := r.ConfigRevert("age2", "", true, nil, nil, mock)
	if ok {
		t.Error("headless with admin required should return false (skipped)")
	}
	if called {
		t.Error("mock should not be called when headless requires admin")
	}
	err, stored := RevertConfigStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) == 0 {
		t.Error("store should be kept when headless skip")
	}
}

func TestConfigRevert_OptionsFnForwarded(t *testing.T) {
	tempRevertStore(t)
	opts := NewConfigRevertFlagOptions()
	opts.IPs = true
	opts.HostFilePath = "/tmp/hosts"
	flags := opts.Flags()
	if err := RevertConfigStore.Store(flags); err != nil {
		t.Fatal(err)
	}

	seen := false
	mock := func(f []string, bin bool, out io.Writer, fn func(*exec.Options)) *exec.Result {
		if fn != nil {
			o := exec.Options{File: "dummy"}
			fn(&o)
			if o.File == "mutated" {
				seen = true
			}
		}
		return &exec.Result{}
	}
	optionsFn := func(o *exec.Options) { o.File = "mutated" }
	r := newReverter(deps{isAdmin: func() bool { return true }, agentRunning: func(bool) bool { return false }})
	ok := r.ConfigRevert("age2", "", false, nil, optionsFn, mock)
	if !ok {
		t.Error("expected success")
	}
	if !seen {
		t.Error("optionsFn was not forwarded to runRevert")
	}
}

func TestRevertRequiresAdminElevation_ParseError(t *testing.T) {
	r := newReverter(deps{isAdmin: func() bool { return false }, agentRunning: func(bool) bool { return false }})
	if !r.RevertRequiresAdminElevation([]string{"--invalid-flag-xyz"}, false) {
		t.Error("expected true on parse error")
	}
}

func TestRequiresAdminElevation_Combos(t *testing.T) {
	t.Parallel()
	tests := []struct {
		isAdmin bool
		agent   bool
		want    bool
	}{
		{true, false, false},
		{false, true, false},
		{false, false, true},
		{true, true, false},
	}
	for _, tc := range tests {
		r := newReverter(deps{isAdmin: func() bool { return tc.isAdmin }, agentRunning: func(bin bool) bool { return tc.agent }})
		got := r.RequiresAdminElevation(false)
		if got != tc.want {
			t.Errorf("isAdmin=%v agent=%v got %v want %v", tc.isAdmin, tc.agent, got, tc.want)
		}
	}
}

func TestRunRevert_BuildsCorrectArgs(t *testing.T) {
	var captured exec.Options
	r := newReverter(deps{exec: func(o exec.Options) *exec.Result {
		captured = o
		return &exec.Result{}
	}})

	flags := []string{"--ip", "--hostFilePath=/tmp/h"}
	result := r.RunRevert(flags, false, nil, nil)
	if result == nil {
		t.Fatal("result nil")
	}
	if captured.File == "" {
		t.Error("File should be set")
	}
	if len(captured.Args) == 0 || captured.Args[0] != ConfigRevertCmd {
		t.Errorf("Args should start with %q, got %v", ConfigRevertCmd, captured.Args)
	}
	found := false
	for _, a := range captured.Args {
		if a == "--ip" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --ip in args, got %v", captured.Args)
	}
}

func TestRunRevert_OptionsFnMutates(t *testing.T) {
	r := newReverter(deps{exec: func(o exec.Options) *exec.Result {
		if o.File != "mutated" {
			t.Errorf("optionsFn mutation not applied, File=%q", o.File)
		}
		return &exec.Result{}
	}})
	r.RunRevert([]string{"--ip"}, false, nil, func(o *exec.Options) {
		o.File = "mutated"
	})
}
