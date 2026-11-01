package user

import (
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

var storyMissions = []string{
	"Mission_Season0_L0P0C1M1",
	"Mission_Season0_L0P0C2M1",
	"Mission_Season0_L0P0C3M1",
	"Mission_Season0_L0P0C4M1",
	"Mission_Season0_L0P0C5M1",
	"Mission_Season0_L0P0C6M1",
	"Mission_Season0_L0P0C6M2",
	"Mission_Season0_L0P0C7M3",
	"Mission_Season0_L0P0C7M2",
	"Mission_Season0_L0P0C7M1",
	"Mission_Season0_L0P0C8M2",
	"Mission_Season0_L0P0C8M1",
	"Mission_Season0_L0P0C9M3",
	"Mission_Season0_L0P0C9M1",
	"Mission_Season0_L0P0C9M2",
	"Mission_Season0_L0P0C10M1",
	"Mission_Season0_L0P1C1M1",
	"Mission_Season0_L0P1C2M1",
	"Mission_Season0_L0P1C2M2",
	"Mission_Season0_L0P1C3M1",
	"Mission_Season0_L0P1C3M2",
	"Mission_Season0_L0P1C3M3",
	"Mission_Season0_L0P1C4M2",
	"Mission_Season0_L0P1C4M1",
	"Mission_Season0_L0P1C5M1",
	"Mission_Season0_L0P1C6M2",
	"Mission_Season0_L0P1C6M1",
	"Mission_Season0_L0P1C6M3",
	"Mission_Season0_L0P1C7M1",
	"Mission_Season0_L0P1C7M2",
	"Mission_Season0_L0P1C8M2",
	"Mission_Season0_L0P1C8M3",
	"Mission_Season0_L0P1C9M2",
	"Mission_Season0_L0P1C9M1",
	"Mission_Season0_L0P1C10M1",
}

type Users struct {
	*models.MainUsers
}

func (users *Users) Initialize() {
	users.MainUsers = &models.MainUsers{
		GenerateFn: users.Generate,
	}
	users.MainUsers.Initialize()
}

func (users *Users) Generate(_ string, persistentData *models.PersistentStringJsonMap, itemDefinitions models.Items, avatarStatsDefinitions models.AvatarStatDefinitions, identifier string, isXbox bool, platformUserId uint64, alias string) models.User {
	// A playfab document that cannot be opened leaves the field nil and the
	// player still logs in, rather than failing the whole account over it.
	playfabData := models.OpenPersistentJsonData[*Data](
		persistentData,
		identifier,
		"playfab",
		NewAvatarStatsUpgradableDefaultData(),
	)
	mainUser := users.MainUsers.Generate(game.AoM, persistentData, itemDefinitions, avatarStatsDefinitions, identifier, isXbox, platformUserId, alias)
	main, ok := mainUser.(*models.MainUser)
	if !ok {
		logger.Fail("User %s: could not be created", identifier)
		return nil
	}
	return &User{
		MainUser:    main,
		PlayfabData: playfabData,
	}
}

type User struct {
	*models.MainUser
	PlayfabData *models.PersistentJsonData[*Data]
}
