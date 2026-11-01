package gameLogs

import (
	"log"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/launcher-common/userData"
)

type Game interface {
	CreateLogs(path string) (err error)
}

var gameIdToGame = map[string]Game{
	game.AoE1: GameAoE1{},
	game.AoE2: GameAoE2{},
	game.AoE3: GameAoE3{},
	game.AoE4: GameAoE4{},
	game.AoM:  GameAoM{},
}

func CreateLogs(gameId string, path string) (err error) {
	finalPath := userData.NewPath(path, gameId).String()
	log.Printf("Creating logs for game %s at path %s", gameId, finalPath)
	return gameIdToGame[gameId].CreateLogs(finalPath)
}
