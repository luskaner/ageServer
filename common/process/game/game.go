package game

import (
	commonGame "github.com/luskaner/ageServer/common/game"
)

var gameToSteamProcess map[string]string
var steamProcessToGame map[string]string

func init() {
	gameToSteamProcess = map[string]string{
		commonGame.AoE1: "AoEDE_s.exe",
		commonGame.AoE2: "AoE2DE_s.exe",
		commonGame.AoE3: "AoE3DE_s.exe",
		commonGame.AoE4: "RelicCardinal.exe",
		commonGame.AoM:  "AoMRT_s.exe",
	}
}

func steamProcess(gameId string) string {
	return gameToSteamProcess[gameId]
}
