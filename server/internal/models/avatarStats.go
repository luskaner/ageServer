package models

import (
	"encoding/json"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

type AvatarStat struct {
	StatID int32 `json:"-"`
	// Read Value with AvatarStats.WithReadLock
	Value int64
	// Read Metadata with AvatarStats.WithReadLock
	Metadata json.RawMessage `json:",omitempty"`
	// Read Metadata with AvatarStats.WithReadLock
	LastUpdated time.Time
}

func NewAvatarStat(statID int32, value int64) AvatarStat {
	return AvatarStat{
		StatID:      statID,
		Value:       value,
		LastUpdated: time.Now().UTC(),
	}
}

func (as *AvatarStat) SetValue(value int64) {
	as.Value = value
	as.LastUpdated = time.Now().UTC()
}

func (as *AvatarStat) Encode(profileID int32) i.A {
	return i.A{
		as.StatID,
		profileID,
		as.Value,
		as.Metadata,
		as.LastUpdated.Unix(),
	}
}

type AvatarStats struct {
	values *i.SafeMap[int32, AvatarStat]
	locks  *i.KeyRWMutex[int32]
}

func (as *AvatarStats) MarshalJSON() ([]byte, error) {
	data := make(map[int32]AvatarStat, as.values.Len())
	for stat := range as.values.Values() {
		data[stat.StatID] = stat
	}
	return json.Marshal(data)
}

func (as *AvatarStats) UnmarshalJSON(b []byte) error {
	var data map[int32]AvatarStat
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	as.values = i.NewSafeMap[int32, AvatarStat]()
	as.locks = i.NewKeyRWMutex[int32]()
	for id, stat := range data {
		stat.StatID = id
		as.values.Store(stat.StatID, stat, func(stored AvatarStat) bool {
			return true
		})
	}
	return nil
}

func (as *AvatarStats) GetStat(id int32) (AvatarStat, bool) {
	return as.values.Load(id)
}

// AddStat Must be ensured that the stat does not already exist
func (as *AvatarStats) AddStat(avatarStat AvatarStat) {
	as.values.Store(avatarStat.StatID, avatarStat, func(stored AvatarStat) bool {
		return false
	})
}

func (as *AvatarStats) Encode(profileId int32) i.A {
	result := i.A{}
	for val := range as.values.Values() {
		result = append(result, val.Encode(profileId))
	}
	return result
}

func newAvatarStats(values map[int32]int64) *AvatarStats {
	avatarStats := &AvatarStats{
		values: i.NewSafeMap[int32, AvatarStat](),
		locks:  i.NewKeyRWMutex[int32](),
	}
	for id, value := range values {
		avatarStats.values.Store(id, AvatarStat{
			StatID:      id,
			Value:       value,
			LastUpdated: time.Now().UTC(),
		}, func(stored AvatarStat) bool {
			return true
		})
	}
	return avatarStats
}

type AvatarStatsUpgradableDefaultData struct {
	InitialUpgradableDefaultData[*AvatarStats]
	gameId                 string
	avatarStatsDefinitions AvatarStatDefinitions
}

func NewAvatarStatsUpgradableDefaultData(gameId string, definitions AvatarStatDefinitions) *AvatarStatsUpgradableDefaultData {
	return &AvatarStatsUpgradableDefaultData{
		InitialUpgradableDefaultData: InitialUpgradableDefaultData[*AvatarStats]{},
		gameId:                       gameId,
		avatarStatsDefinitions:       definitions,
	}
}

func (a *AvatarStatsUpgradableDefaultData) Default() *AvatarStats {
	var values map[string]int64
	switch a.gameId {
	case game.AoE2:
		// TODO: Remove the ones not needed
		values = map[string]int64{
			"STAT_NUM_MVP_AWARDS":           0,
			"STAT_HIGHEST_SCORE_TOTAL":      0,
			"STAT_HIGHEST_SCORE_ECONOMIC":   0,
			"STAT_HIGHEST_SCORE_TECHNOLOGY": 0,
			"STAT_CAREER_UNITS_KILLED":      0,
			"STAT_CAREER_UNITS_LOST":        0,
			"STAT_CAREER_UNITS_CONVERTED":   0,
			"STAT_CAREER_BUILDINGS_RAZED":   0,
			"STAT_CAREER_BUILDINGS_LOST":    0,
			"STAT_CAREER_NUM_CASTLES":       0,
			"STAT_GAMES_PLAYED_ONLINE":      0,
			"STAT_ELO_XRM_WINS":             0,
			"STAT_POP_CAP_200_MP":           0,
			"STAT_POP_PEAK_200_MP":          0,
			"STAT_TOTAL_GAMES":              0,
		}
	// TODO: What benefits Aoe4 here ?
	case game.AoE3:
		// FIXME: Is this even needed?
		values = map[string]int64{
			"STAT_EVENT_EXPLORER_SKIN_CHALLENGE_14c": 16,
		}
	case game.AoM:
		values = map[string]int64{
			"STAT_GAUNTLET_REWARD_XP":     2_147_483_647,
			"STAT_GAUNTLET_REWARD_FAVOUR": 19_500,
		}
	default:
		values = map[string]int64{}
	}
	intValues := make(map[int32]int64, len(values))
	for k, v := range values {
		if id, ok := a.avatarStatsDefinitions.GetIdByName(k); ok {
			intValues[id] = v
		}
	}
	return newAvatarStats(intValues)
}
