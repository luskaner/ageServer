package steam

import (
	"testing"

	"github.com/luskaner/ageServer/common/game/steam"
)

func TestString(t *testing.T) {
	g := &steam.Game{}
	e := Exec{Game: g}
	if got := e.String(); got != "Steam" {
		t.Errorf("String() = %q, want %q", got, "Steam")
	}
}

func TestGameProcesses(t *testing.T) {
	e := Exec{}
	steamProcess, steamMacOsNative, xboxProcess := e.GameProcesses()
	if !steamProcess {
		t.Error("steamProcess should be true")
	}
	if steamMacOsNative {
		t.Error("steamMacOsNative should be false on Windows")
	}
	if xboxProcess {
		t.Error("xboxProcess should be false")
	}
}

func TestNewExecFromGameNil(t *testing.T) {
	e, ok := NewExecFromGame(nil)
	if ok {
		t.Error("NewExecFromGame(nil) should return ok=false")
	}
	if e != nil {
		t.Error("exec should be nil")
	}
}

func TestNewExecFromGameValid(t *testing.T) {
	g := &steam.Game{}
	e, ok := NewExecFromGame(g)
	if !ok {
		t.Error("NewExecFromGame should return ok=true")
	}
	if e == nil {
		t.Fatal("exec should not be nil")
	}
	if e.Game != g {
		t.Error("exec.Game should be the same as input")
	}
}

func TestNewExec(t *testing.T) {
	// NewExec will likely fail since Steam is not installed
	_, ok := NewExec("age2")
	// Just verify it doesn't panic
	_ = ok
}
