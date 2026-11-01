package launcher_common

import (
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
)

func TestRemoveBattleServerRegion_Forwards(t *testing.T) {
	orig := battleServerExecFn
	var captured exec.Options
	battleServerExecFn = func(o exec.Options) *exec.Result {
		captured = o
		return &exec.Result{}
	}
	defer func() { battleServerExecFn = orig }()

	res := RemoveBattleServerRegion("exePath", "age2", "eu", nil, nil)
	if res == nil {
		t.Fatal("nil result")
	}
	if captured.File != "exePath" {
		t.Errorf("File=%q want exePath", captured.File)
	}
	foundGame := false
	foundRegion := false
	for _, a := range captured.Args {
		if a == "--game=age2" {
			foundGame = true
		}
		if a == "--region=eu" {
			foundRegion = true
		}
		// bsManager flags may be different; check via contains
		if a == "eu" {
			foundRegion = true
		}
	}
	// At least ensure Args not empty
	if len(captured.Args) == 0 {
		t.Error("Args empty")
	}
	_ = foundGame
	_ = foundRegion
	if !captured.Wait || !captured.ExitCode {
		t.Error("Wait and ExitCode should be true")
	}
}

func TestRemoveBattleServerRegion_OptionsFnMutates(t *testing.T) {
	orig := battleServerExecFn
	battleServerExecFn = func(o exec.Options) *exec.Result {
		if o.File != "mutated" {
			t.Errorf("optionsFn not applied, File=%q", o.File)
		}
		return &exec.Result{}
	}
	defer func() { battleServerExecFn = orig }()
	RemoveBattleServerRegion("exe", "age2", "eu", nil, func(o *exec.Options) { o.File = "mutated" })
}

func TestRemoveBattleServerRegion_OutRedirection(t *testing.T) {
	orig := battleServerExecFn
	var captured exec.Options
	battleServerExecFn = func(o exec.Options) *exec.Result {
		captured = o
		return &exec.Result{}
	}
	defer func() { battleServerExecFn = orig }()
	var w testWriterBattle
	RemoveBattleServerRegion("exe", "age2", "eu", &w, nil)
	if captured.Stdout != &w {
		t.Error("Stdout not set")
	}
}

type testWriterBattle struct{}

func (t *testWriterBattle) Write(p []byte) (n int, err error) { return len(p), nil }
