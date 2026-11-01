package game

import (
	mapset "github.com/deckarep/golang-set/v3"
	commonGame "github.com/luskaner/ageServer/common/game"
)

var gameToNativeSteamProcess map[string]string
var nativeSteamProcessToGame map[string]string

func init() {
	gameToNativeSteamProcess = map[string]string{
		commonGame.AoE2: "Age Of Empires II",
	}
}

func nativeSteamProcess(gameId string) string {
	return gameToNativeSteamProcess[gameId]
}

func Game(process string, macOsNative bool) (gameId string) {
	if macOsNative {
		if nativeSteamProcessToGame == nil {
			nativeSteamProcessToGame = make(map[string]string, len(gameToNativeSteamProcess))
			for curGameId, procName := range gameToNativeSteamProcess {
				nativeSteamProcessToGame[procName] = curGameId
			}
		}
		if g, ok := nativeSteamProcessToGame[process]; ok {
			return g
		}
		return
	}
	return game(process)
}

func Processes(gameId string, steam bool, steamMacOsNative bool, _ bool) []string {
	processes := mapset.NewThreadUnsafeSet[string]()
	if steam {
		processes.Add(steamProcess(gameId))
	}
	if steamMacOsNative {
		processes.Add(nativeSteamProcess(gameId))
	}
	return processes.ToSlice()
}
