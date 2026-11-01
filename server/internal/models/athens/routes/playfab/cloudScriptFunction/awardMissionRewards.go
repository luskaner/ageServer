package cloudScriptFunction

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/athens/user"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

type AwardMissionRewardsParameters struct {
	CdnPath    string
	Difficulty string
	MissionId  string
	Won        bool
}

type AwardMissionRewardsResultItemsAdded struct {
	Amount         int    `json:"amount"`
	ItemFriendlyId string `json:"itemFriendlyId"`
}

type AwardMissionRewardsResult struct {
	ItemsAdded []AwardMissionRewardsResultItemsAdded `json:"itemsAdded"`
}

type AwardMissionRewardsFunction struct {
	*playfab.CloudScriptFunctionBase[AwardMissionRewardsParameters, AwardMissionRewardsResult]
}

func (a *AwardMissionRewardsFunction) RunTyped(_ models.Game, u models.User, parameters *AwardMissionRewardsParameters) *AwardMissionRewardsResult {
	result := &AwardMissionRewardsResult{
		ItemsAdded: []AwardMissionRewardsResultItemsAdded{},
	}
	if elements := strings.Split(parameters.MissionId, "_"); elements[1] == "Gauntlet" {
		actualMissionId := strings.Join(elements[2:], "_")
		athensUser := u.(*user.User)
		_ = athensUser.PlayfabData.WithReadWrite(func(data *user.Data) error {
			challenge := data.Challenge
			progress := challenge.Progress
			if progress == nil {
				return fmt.Errorf("no challenge progress found")
			}
			progressValue := progress.Value
			if progressValue.MissionBeingPlayedRightNow != parameters.MissionId {
				return fmt.Errorf("challenge progress not correct")
			}
			if parameters.Won {
				progressValue.CompletedMissions = append(progressValue.CompletedMissions, parameters.MissionId)
				for _, mission := range (*challenge.Labyrinth.Value).Missions {
					if mission.Id != actualMissionId {
						continue
					}
					for _, reward := range mission.Rewards {
						elements = strings.Split(reward.ItemId, "_")
						rarity, _ := strconv.Atoi(elements[3])
						progressValue.Inventory = append(progressValue.Inventory, user.ProgressInventory{
							SeasonId: "Gauntlet",
							Item:     elements[2],
							Rarity:   rarity,
						})
						result.ItemsAdded = append(result.ItemsAdded, AwardMissionRewardsResultItemsAdded{
							Amount:         reward.Amount,
							ItemFriendlyId: reward.ItemId,
						})
					}
				}
				progressValue.MissionBeingPlayedRightNow = ""
				progress.UpdateLastUpdated()
				data.DataVersion++
				return nil
			}
			return fmt.Errorf("no rewards awarded for lost mission")
		})
	}
	return result
}

func (a *AwardMissionRewardsFunction) Name() string {
	return "AwardMissionRewards"
}

func NewAwardMissionRewardsFunction() *AwardMissionRewardsFunction {
	f := &AwardMissionRewardsFunction{}
	f.CloudScriptFunctionBase = playfab.NewCloudScriptFunctionBase[AwardMissionRewardsParameters, AwardMissionRewardsResult](f)
	return f
}
