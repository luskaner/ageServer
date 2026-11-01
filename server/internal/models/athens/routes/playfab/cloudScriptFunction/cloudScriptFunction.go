package cloudScriptFunction

import (
	"github.com/luskaner/ageServer/server/internal/models/athens/routes/playfab/cloudScriptFunction/buildGauntletLabyrinth"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

var Store map[string]playfab.CloudScriptFunction
var fns = []playfab.CloudScriptFunction{
	NewAwardMissionRewardsFunction(),
	NewStartGauntletMissionFunction(),
	buildGauntletLabyrinth.NewFunction(),
}

func init() {
	Store = make(map[string]playfab.CloudScriptFunction, len(fns))
	for _, fn := range fns {
		Store[fn.Name()] = fn
	}
}
