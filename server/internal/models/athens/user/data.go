package user

import (
	"time"

	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/playfab/data"
)

type PunchCardProgress struct {
	Holes                     uint8
	DateOfMostRecentHolePunch data.CustomTime
}

type MissionRewards struct {
	Amount  int
	Scaling string
	ItemId  string
}

type WorldTwist struct {
	Id              string
	ArenaEffectName string
	Title           string
	Description     string
	OwnerIcon       string
	OwnerPortrait   string
	Visualization   string
}

type Opponent struct {
	Civ              string
	Team             int
	Personality      string
	DifficultyOffset int
}

type ChallengeMission struct {
	RowIndex                int `json:"-"`
	Id                      string
	Predecessors            []string
	PositionX               int
	PositionY               int
	Visualization           string
	Map                     string
	Size                    string
	VictoryCondition        string
	GameType                string
	MapVisibility           string
	StartingResources       string
	AllowTitans             bool
	WorldTwists             []WorldTwist
	Opponents               []Opponent
	OpponentsFor2PlayerCoop []Opponent
	Rewards                 []MissionRewards
	MinimapImage            string
	MapPreviewImage         string
}

type Labyrinth struct {
	Id         int
	Difficulty string
	Missions   []ChallengeMission
}

type ProgressInventory struct {
	SeasonId string
	Item     string
	Rarity   int
}

type Progress struct {
	Lives                      int
	CompletedMissions          []string
	Inventory                  []ProgressInventory
	MissionBeingPlayedRightNow string
}

type Challenge struct {
	Labyrinth *data.BaseValue[Labyrinth]
	Progress  *data.BaseValue[Progress]
}

type StoryMission struct {
	State                 string
	RewardsAwarded        string
	CompletionCountEasy   uint32
	CompletionCountMedium uint32
	CompletionCountHard   uint32
}

type Data struct {
	Challenge         Challenge
	StoryMissions     map[string]data.BaseValue[StoryMission]
	PunchCardProgress data.BaseValue[PunchCardProgress]
	DataVersion       uint32
}

type PlayfabUpgradableDefaultData struct {
	models.InitialUpgradableDefaultData[*Data]
}

func NewAvatarStatsUpgradableDefaultData() *PlayfabUpgradableDefaultData {
	return &PlayfabUpgradableDefaultData{}
}

func (p *PlayfabUpgradableDefaultData) Default() *Data {
	lastUpdated := data.CustomTime{Time: time.Now(), Format: "2006-01-02T15:04:05.000Z"}
	permission := "Private"
	missions := make(map[string]data.BaseValue[StoryMission], len(storyMissions))
	for _, missionId := range storyMissions {
		missions[missionId] = data.BaseValue[StoryMission]{
			LastUpdated: lastUpdated,
			Permission:  permission,
			Value: &StoryMission{
				State:               "Completed",
				RewardsAwarded:      "Hard",
				CompletionCountHard: 1,
			},
		}
	}
	return &Data{
		StoryMissions: missions,
		PunchCardProgress: data.BaseValue[PunchCardProgress]{
			LastUpdated: lastUpdated,
			Permission:  permission,
			Value: &PunchCardProgress{
				DateOfMostRecentHolePunch: data.CustomTime{
					Time:   time.Date(2024, 5, 2, 3, 34, 0, 0, time.UTC),
					Format: time.RFC3339,
				},
			},
		},
		DataVersion: 0,
	}
}
