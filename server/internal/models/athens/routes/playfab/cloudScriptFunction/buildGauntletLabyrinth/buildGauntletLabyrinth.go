package buildGauntletLabyrinth

import (
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens"
	"github.com/luskaner/ageServer/server/internal/models/athens/user"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	userData "github.com/luskaner/ageServer/server/internal/models/playfab/data"
)

type Result struct {
	Labyrinth *user.Labyrinth
	Progress  *user.Progress
}

type Parameters struct {
	CdnPath            string
	GauntletDifficulty string
}

type Function struct {
	*playfab.CloudScriptFunctionBase[Parameters, Result]
}

func (b *Function) RunTyped(game models.Game, u models.User, parameters *Parameters) *Result {
	athensGame, ok := game.(*athens.Game)
	if !ok {
		return &Result{}
	}
	athensUser := u.(*user.User)
	nodes := GenerateNumberOfNodes()
	nodeRows := GenerateNodeRows(nodes)
	blessings := RandomizedBlessings(athensGame.AllowedBlessings)
	poolIndexes := athensGame.GauntletPoolIndexByDifficulty[parameters.GauntletDifficulty]
	missionColumns := GenerateMissions(nodeRows, poolIndexes, athensGame.GauntletMissionPools, blessings)
	var missions []user.ChallengeMission
	for _, missionColumn := range missionColumns {
		for _, mission := range missionColumn {
			missions = append(missions, *mission)
		}
	}
	var result *Result
	_ = athensUser.PlayfabData.WithReadWrite(func(data *user.Data) error {
		var id int
		if labyrinth := data.Challenge.Labyrinth; labyrinth != nil {
			id = (*labyrinth.Value).Id + 1
		} else {
			id = 1
		}
		data.Challenge = user.Challenge{
			Labyrinth: userData.NewPrivateBaseValue(user.Labyrinth{
				Id:         id,
				Difficulty: parameters.GauntletDifficulty,
				Missions:   missions,
			}),
			Progress: userData.NewPrivateBaseValue(user.Progress{
				Lives:             3,
				CompletedMissions: []string{},
				Inventory:         []user.ProgressInventory{},
			}),
		}
		data.DataVersion++
		result = &Result{
			Labyrinth: data.Challenge.Labyrinth.Value,
			Progress:  data.Challenge.Progress.Value,
		}
		return nil
	})
	return result
}

func (b *Function) Name() string {
	return "BuildGauntletLabyrinth"
}

func NewFunction() *Function {
	f := &Function{}
	f.CloudScriptFunctionBase = playfab.NewCloudScriptFunctionBase[Parameters, Result](f)
	return f
}
