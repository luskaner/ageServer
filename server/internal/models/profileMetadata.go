package models

import (
	"github.com/luskaner/ageServer/common/game"
)

type AvatarMetadataUpgradableDefaultData struct {
	InitialUpgradableDefaultData[*string]
	gameId string
}

func NewAvatarMetadataUpgradableDefaultData(gameId string) *AvatarMetadataUpgradableDefaultData {
	return &AvatarMetadataUpgradableDefaultData{
		InitialUpgradableDefaultData: InitialUpgradableDefaultData[*string]{},
		gameId:                       gameId,
	}
}

func (p *AvatarMetadataUpgradableDefaultData) Default() *string {
	var metadata string
	switch p.gameId {
	case game.AoE3, game.AoM:
		metadata = `{"v":1,"twr":0,"wlr":0,"ai":1,"ac":0}`
	case game.AoE4:
		metadata = `{"sharedHistory":1,"hardwareType":0,"inputDeviceType":0}`
	}
	return &metadata
}
