package internal

import commonGame "github.com/luskaner/ageServer/common/game"

var (
	V190 uint16 = 190
	V193 uint16 = 193
	V194 uint16 = 194
)

func SinceTheBalticPowers(game string, clientLibVersion uint16) bool {
	return game == commonGame.AoE3 && clientLibVersion >= V194
}

func BeforeTheBalticPowers(game string, clientLibVersion uint16) bool {
	return game == commonGame.AoE3 && clientLibVersion < V194
}
