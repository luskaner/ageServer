//go:build windows

package game

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestXboxProcess(t *testing.T) {
	tests := []struct {
		gameId string
		want   string
	}{
		{game.AoE1, "AoEDE.exe"},
		{game.AoE2, "AoE2DE.exe"},
		{game.AoE3, "AoE3DE.exe"},
		{game.AoE4, "RelicCardinal_ws.exe"},
		{game.AoM, "AoMRT.exe"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.gameId, func(t *testing.T) {
			if got := xboxProcess(tt.gameId); got != tt.want {
				t.Errorf("xboxProcess(%q) = %q, want %q", tt.gameId, got, tt.want)
			}
		})
	}
}

func TestGame_XboxProcess(t *testing.T) {
	tests := []struct {
		process string
		want    string
	}{
		{"AoE2DE.exe", game.AoE2},
		{"AoEDE.exe", game.AoE1},
		{"AoMRT.exe", game.AoM},
	}
	for _, tt := range tests {
		t.Run(tt.process, func(t *testing.T) {
			if got := Game(tt.process, false); got != tt.want {
				t.Errorf("Game(%q, false) = %q, want %q", tt.process, got, tt.want)
			}
		})
	}
}

func TestGame_SteamProcess(t *testing.T) {
	tests := []struct {
		process string
		want    string
	}{
		{"AoE2DE_s.exe", game.AoE2},
		{"AoEDE_s.exe", game.AoE1},
		{"AoE3DE_s.exe", game.AoE3},
		{"RelicCardinal.exe", game.AoE4},
		{"AoMRT_s.exe", game.AoM},
		{"unknown.exe", ""},
	}
	for _, tt := range tests {
		t.Run(tt.process, func(t *testing.T) {
			if got := Game(tt.process, false); got != tt.want {
				t.Errorf("Game(%q, false) = %q, want %q", tt.process, got, tt.want)
			}
		})
	}
}

func TestProcesses_Xbox(t *testing.T) {
	procs := Processes(game.AoE2, false, false, true)
	if len(procs) != 1 {
		t.Fatalf("expected 1 process, got %d: %v", len(procs), procs)
	}
	if procs[0] != "AoE2DE.exe" {
		t.Errorf("expected AoE2DE.exe, got %s", procs[0])
	}
}

func TestProcesses_Both(t *testing.T) {
	procs := Processes(game.AoE2, true, false, true)
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes, got %d: %v", len(procs), procs)
	}
}
