package cmd

import (
	"errors"
	"testing"

	"battle-server-manager/internal"
	"battle-server-manager/internal/cmdUtils"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
)

func TestRunClean_SyntaxError(t *testing.T) {
	_, code := runClean([]string{"--unknown-flag"})
	if code != common.ErrSyntax {
		t.Fatalf("expected ErrSyntax, got %d", code)
	}
}

func TestRunClean_Success(t *testing.T) {
	orig := removeAllFnClean
	defer func() { removeAllFnClean = orig }()
	called := false
	removeAllFnClean = func(onlyInvalid bool) (error, int) {
		if !onlyInvalid {
			t.Error("onlyInvalid should be true")
		}
		called = true
		return nil, 0
	}
	_, code := runClean([]string{})
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !called {
		t.Error("RemoveAll not called")
	}
}

func TestRunRemove_SyntaxError(t *testing.T) {
	_, code := runRemove([]string{"--unknown"})
	if code != common.ErrSyntax {
		t.Fatalf("expected ErrSyntax, got %d", code)
	}
}

func TestRunRemove_ParsedGameIdsError(t *testing.T) {
	orig := parsedGameIdsFnRemove
	defer func() { parsedGameIdsFnRemove = orig }()
	parsedGameIdsFnRemove = func(*[]string) (mapset.Set[string], error) {
		return nil, errors.New("bad")
	}
	_, code := runRemove([]string{"--games", "age2", "--region", "eu"})
	if code != internal.ErrGames {
		t.Fatalf("expected ErrGames, got %d", code)
	}
}

func TestRunRemove_SuccessNoConfigs(t *testing.T) {
	origParsed := parsedGameIdsFnRemove
	origConfigs := battleServerConfigsFn
	origRemove := removeFn
	defer func() {
		parsedGameIdsFnRemove = origParsed
		battleServerConfigsFn = origConfigs
		removeFn = origRemove
	}()
	parsedGameIdsFnRemove = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	battleServerConfigsFn = func(gameId string, onlyValid, ignorePid bool) ([]battleServer.Config, error) {
		return nil, errors.New("read fail")
	}
	// Should not return error, just continue and return 0
	_, code := runRemove([]string{"--games", "age2", "--region", "eu"})
	if code != 0 {
		t.Fatalf("expected 0 when Configs fails but continues, got %d", code)
	}
	// Test with valid config and Remove returns false
	battleServerConfigsFn = func(string, bool, bool) ([]battleServer.Config, error) {
		return []battleServer.Config{{Base: battleServer.Base{Region: "eu"}}}, nil
	}
	removeFn = func(string, []battleServer.Config, bool) bool { return false }
	_, code = runRemove([]string{"--games", "age2", "--region", "eu"})
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	// Test with Remove true
	removeFn = func(string, []battleServer.Config, bool) bool { return true }
	_, code = runRemove([]string{"--games", "age2", "--region", "eu"})
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}

func TestRunRemoveAll_SyntaxError(t *testing.T) {
	_, code := runRemoveAll([]string{"--unknown"})
	if code != common.ErrSyntax {
		t.Fatalf("expected ErrSyntax, got %d", code)
	}
}

func TestRunRemoveAll_Success(t *testing.T) {
	orig := removeAllFn
	defer func() { removeAllFn = orig }()
	called := false
	removeAllFn = func(onlyInvalid bool) (error, int) {
		if onlyInvalid {
			t.Error("onlyInvalid should be false")
		}
		called = true
		return nil, 0
	}
	_, code := runRemoveAll([]string{})
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !called {
		t.Error("not called")
	}
}

func TestExecute_LockFail(t *testing.T) {
	origLock := newPidLock
	defer func() { newPidLock = origLock }()
	newPidLock = func() pidLocker {
		return &fakeLocker{lockErr: errors.New("lock fail")}
	}
	_, code := Execute()
	if code != common.ErrPidLock {
		t.Fatalf("expected ErrPidLock, got %d", code)
	}
}

func TestExecute_Success(t *testing.T) {
	origLock := newPidLock
	defer func() { newPidLock = origLock }()
	newPidLock = func() pidLocker { return &fakeLocker{} }
	Version = "test"
	_, code := Execute()
	if code == common.ErrPidLock {
		t.Errorf("should not be ErrPidLock, got %d", code)
	}
	_ = cmdUtils.GameIds
}

type fakeLocker struct {
	lockErr   error
	unlockErr error
}

func (f *fakeLocker) Lock() error   { return f.lockErr }
func (f *fakeLocker) Unlock() error { return f.unlockErr }

type mockRootFlagSet struct {
	executeFn func(string) (error, int)
}

func (m *mockRootFlagSet) Execute(version string) (error, int) {
	return m.executeFn(version)
}
func (m *mockRootFlagSet) RegisterCommand(name string, fn func([]string) (error, int)) {}
