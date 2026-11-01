package game

import mapset "github.com/deckarep/golang-set/v3"

type Locatable interface {
	Path() string
}

const (
	AoE1 = "age1"
	AoE2 = "age2"
	AoE3 = "age3"
	AoE4 = "age4"
	AoM  = "athens"
)

var SupportedGames = mapset.NewThreadUnsafeSet[string](AoE1, AoE2, AoE3, AoE4, AoM)
var AllGames = SupportedGames
