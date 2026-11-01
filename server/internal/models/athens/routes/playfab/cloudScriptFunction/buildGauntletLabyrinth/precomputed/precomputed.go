package precomputed

import (
	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/server/internal/models/athens/routes/playfab"
)

type Blessing struct {
	EffectName string
	Rarity     int
}

func (a Blessing) GauntletItem() string {
	return playfab.ItemName("Gauntlet", a.EffectName, a.Rarity)
}

func blessingKeys(blessings []playfab.Blessings) (keys mapset.Set[Blessing]) {
	keys = mapset.NewThreadUnsafeSet[Blessing]()
	for _, blessing := range blessings {
		// This assumes empty KnownRarities means all rarities are included
		for _, rarity := range blessing.KnownRarities {
			keys.Add(Blessing{blessing.EffectName, rarity})
		}
	}
	return
}

func AllowedGauntletBlessings(gauntlet playfab.Gauntlet, knownBlessings []playfab.Blessings) (blessings map[int][]string) {
	disallowedBlessings := blessingKeys(gauntlet.Rewards.ExcludeFromRegularRewards)
	allBlessings := blessingKeys(knownBlessings)
	allowedBlessings := allBlessings.Difference(disallowedBlessings)
	blessingLevels := mapset.NewThreadUnsafeSet[int]()
	for blessing := range allowedBlessings.Iter() {
		blessingLevels.Add(blessing.Rarity)
	}
	blessings = make(map[int][]string, blessingLevels.Cardinality())
	for level := range blessingLevels.Iter() {
		blessings[level] = []string{}
	}
	for blessing := range allowedBlessings.Iter() {
		blessings[blessing.Rarity] = append(blessings[blessing.Rarity], blessing.GauntletItem())
	}
	return blessings
}

func PoolNamesToIndex(missionPools playfab.GauntletMissionPools) map[string]int {
	poolNamesToId := make(map[string]int, len(missionPools))
	for index, pool := range missionPools {
		poolNamesToId[pool.Name] = index
	}
	return poolNamesToId
}

func PoolsIndexByDifficulty(gauntlet playfab.Gauntlet, poolNamesToIndex map[string]int) (poolsIndexes map[string][]int) {
	poolsIndexes = make(map[string][]int)
	for _, config := range gauntlet.LabyrinthConfigs {
		poolDifficultyIndex := make([]int, len(config.ColumnConfigs)+1)
		var column int
		var columnConfig playfab.ColumnConfig
		for column, columnConfig = range config.ColumnConfigs {
			poolDifficultyIndex[column] = poolNamesToIndex[columnConfig.MissionPool]
		}
		poolDifficultyIndex[column+1] = poolNamesToIndex[config.BossMissionPool]
		for _, difficulty := range config.ForGauntletDifficulties {
			poolsIndexes[difficulty] = poolDifficultyIndex
		}
	}
	return
}
