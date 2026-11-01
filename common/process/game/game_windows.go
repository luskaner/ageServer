package game

import (
	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
)

var gameToXboxProcess map[string]string
var xboxProcessToGame map[string]string

func init() {
	gameToXboxProcess = map[string]string{
		game.AoE1: "AoEDE.exe",
		game.AoE2: "AoE2DE.exe",
		game.AoE3: "AoE3DE.exe",
		game.AoE4: "RelicCardinal_ws.exe",
		game.AoM:  "AoMRT.exe",
	}
}

func xboxProcess(gameId string) string {
	return gameToXboxProcess[gameId]
}

func Game(process string, _ bool) (gameId string) {
	if steamProcessToGame == nil {
		steamProcessToGame = make(map[string]string, len(gameToSteamProcess))
		for curGameId, procName := range gameToSteamProcess {
			steamProcessToGame[procName] = curGameId
		}
	}
	if g, ok := steamProcessToGame[process]; ok {
		return g
	}
	if xboxProcessToGame == nil {
		xboxProcessToGame = make(map[string]string, len(gameToXboxProcess))
		for curGameId, procName := range gameToXboxProcess {
			xboxProcessToGame[procName] = curGameId
		}
	}
	return xboxProcessToGame[process]
}

func Processes(gameId string, steam bool, _ bool, xbox bool) []string {
	processes := mapset.NewThreadUnsafeSet[string]()
	if steam {
		processes.Add(steamProcess(gameId))
	}
	if xbox {
		processes.Add(xboxProcess(gameId))
	}
	return processes.ToSlice()
}
