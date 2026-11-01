package executor

import (
	"errors"
	"testing"

	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
)

func TestExecuteBattleServer_PortsTooShort(t *testing.T) {
	_, err := ExecuteBattleServer(game.AoE2, "/tmp/bs.exe", "eu", "Test", []int{27015}, "/tmp/cert", "/tmp/key", nil, false, "")
	if err == nil {
		t.Error("expected error for ports too short")
	}
}

func TestExecuteBattleServer_SimulationPeriodPerGame(t *testing.T) {
	orig := execWithOptions
	defer func() { execWithOptions = orig }()

	tests := []struct {
		gameId string
		wantSP string
	}{
		{game.AoE1, "25"},
		{game.AoE3, "50"},
		{game.AoM, "50"},
		{game.AoE2, "125"},
		{game.AoE4, "125"},
		{"unknown", "125"},
	}
	for _, tc := range tests {
		t.Run(tc.gameId, func(t *testing.T) {
			var capturedArgs []string
			execWithOptions = func(_ string, opts *exec.Options) exec.Result {
				capturedArgs = opts.Args
				return exec.Result{Pid: 1234}
			}
			ports := []int{27015, 27016, 27017}
			_, err := ExecuteBattleServer(tc.gameId, "/tmp/bs.exe", "eu", "Test", ports, "/tmp/cert", "/tmp/key", nil, false, "")
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			found := false
			for i, a := range capturedArgs {
				if a == "-simulationPeriod" && i+1 < len(capturedArgs) && capturedArgs[i+1] == tc.wantSP {
					found = true
				}
			}
			if !found {
				t.Fatalf("simulationPeriod %q not found in args %v", tc.wantSP, capturedArgs)
			}
		})
	}
}

func TestExecuteBattleServer_ArgsConstruction(t *testing.T) {
	orig := execWithOptions
	defer func() { execWithOptions = orig }()

	var captured *exec.Options
	var capturedGame string
	execWithOptions = func(gameId string, opts *exec.Options) exec.Result {
		capturedGame = gameId
		captured = opts
		return exec.Result{Pid: 999}
	}

	ports := []int{27015, 27016, -1}
	extra := []string{"--extra", "val"}
	pid, err := ExecuteBattleServer(game.AoE2, "/path/bs.exe", "eu-west", "MyServer", ports, "/cert", "/key", extra, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if pid != 999 {
		t.Fatalf("pid %d want 999", pid)
	}
	if capturedGame != game.AoE2 {
		t.Fatalf("gameId %q", capturedGame)
	}
	if captured.File != "/path/bs.exe" {
		t.Fatalf("File %q", captured.File)
	}
	if !captured.UseWorkingPath {
		t.Error("UseWorkingPath should be true")
	}
	if !captured.ShowWindow {
		t.Error("ShowWindow should be true when hideWindow false")
	}
	// Check args contain expected
	hasRegion := false
	hasOutOfBand := false
	for i, a := range captured.Args {
		if a == "-region" && i+1 < len(captured.Args) && captured.Args[i+1] == "eu-west" {
			hasRegion = true
		}
		if a == "-outOfBandPort" {
			hasOutOfBand = true
		}
		if a == "--extra" {
			// extra args should be appended
			if i+1 >= len(captured.Args) || captured.Args[i+1] != "val" {
				t.Error("extra args not appended")
			}
		}
	}
	if !hasRegion {
		t.Error("region not found")
	}
	if hasOutOfBand {
		t.Error("outOfBand should not be present when -1")
	}
	// With outOfBand not -1, it should be present
	captured = nil
	ports2 := []int{27015, 27016, 27017}
	_, _ = ExecuteBattleServer(game.AoE2, "/path/bs.exe", "eu", "Test", ports2, "/cert", "/key", nil, true, "/logs")
	if captured == nil {
		t.Fatal("no captured")
	}
	hasOutOfBand = false
	for _, a := range captured.Args {
		if a == "-outOfBandPort" {
			hasOutOfBand = true
		}
	}
	if !hasOutOfBand {
		t.Error("outOfBand should be present")
	}
	if captured.ShowWindow {
		t.Error("ShowWindow should be false when hideWindow true")
	}
}

func TestExecuteBattleServer_HideWindowWithLogRoot(t *testing.T) {
	orig := execWithOptions
	defer func() { execWithOptions = orig }()

	execWithOptions = func(_ string, opts *exec.Options) exec.Result {
		if !opts.Pid {
			t.Error("Pid should be true")
		}
		return exec.Result{Pid: 1}
	}
	// Test with hideWindow true and no logRoot to avoid file lock on Windows
	_, err := ExecuteBattleServer(game.AoE2, "/tmp/bs.exe", "eu", "Test", []int{27015, 27016, 27017}, "/c", "/k", nil, true, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Test with hideWindow false and logRoot set - should not create file (only when hideWindow true)
	_, err = ExecuteBattleServer(game.AoE2, "/tmp/bs.exe", "eu", "Test", []int{27015, 27016, 27017}, "/c", "/k", nil, false, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestExecuteBattleServer_ExecFailure(t *testing.T) {
	orig := execWithOptions
	defer func() { execWithOptions = orig }()

	execWithOptions = func(string, *exec.Options) exec.Result {
		return exec.Result{Err: errors.New("exec fail")}
	}
	_, err := ExecuteBattleServer(game.AoE2, "/tmp/bs.exe", "eu", "Test", []int{27015, 27016, 27017}, "/c", "/k", nil, false, "")
	if err == nil || err.Error() != "exec fail" {
		t.Fatalf("expected exec fail, got %v", err)
	}
}

func TestExecuteBattleServer_PortsLength2AppendsMinus1(t *testing.T) {
	orig := execWithOptions
	defer func() { execWithOptions = orig }()

	execWithOptions = func(_ string, opts *exec.Options) exec.Result {
		// Check that outOfBand is not added when ports was len 2 (appended -1)
		for _, a := range opts.Args {
			if a == "-outOfBandPort" {
				t.Error("should not have outOfBand for len 2")
			}
		}
		return exec.Result{Pid: 1}
	}
	_, err := ExecuteBattleServer(game.AoE2, "/tmp/bs.exe", "eu", "Test", []int{27015, 27016}, "/c", "/k", nil, false, "")
	if err != nil {
		t.Fatal(err)
	}
}
