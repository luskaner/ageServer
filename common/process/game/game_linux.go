package game

import mapset "github.com/deckarep/golang-set/v3"

func Game(process string, _ bool) (gameId string) {
	return game(process)
}

func Processes(gameId string, steam bool, _ bool, _ bool) []string {
	processes := mapset.NewThreadUnsafeSet[string]()
	if steam {
		processes.Add(steamProcess(gameId))
	}
	return processes.ToSlice()
}
