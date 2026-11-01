package cloudScriptFunction

import (
	"fmt"

	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens/user"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

type StartGauntletMissionParameters struct {
	CdnPath   string
	MissionId string
}

type StartGauntletMissionResult struct{}

type StartGauntletMissionFunction struct {
	*playfab.CloudScriptFunctionBase[StartGauntletMissionParameters, StartGauntletMissionResult]
}

func (s *StartGauntletMissionFunction) RunTyped(_ models.Game, u models.User, parameters *StartGauntletMissionParameters) *StartGauntletMissionResult {
	athensUser := u.(*user.User)
	_ = athensUser.PlayfabData.WithReadWrite(func(data *user.Data) error {
		progress := data.Challenge.Progress
		if progress == nil {
			return fmt.Errorf("no progress found")
		}
		if (*progress.Value).MissionBeingPlayedRightNow != parameters.MissionId {
			progress.Update(func(progress *user.Progress) {
				progress.MissionBeingPlayedRightNow = parameters.MissionId
			})
			data.DataVersion++
			return nil
		}
		return fmt.Errorf("no updates needed")
	})
	return nil
}

func (s *StartGauntletMissionFunction) Name() string {
	return "StartGauntletMission"
}

func NewStartGauntletMissionFunction() *StartGauntletMissionFunction {
	f := &StartGauntletMissionFunction{}
	f.CloudScriptFunctionBase = playfab.NewCloudScriptFunctionBase[StartGauntletMissionParameters, StartGauntletMissionResult](f)
	return f
}
