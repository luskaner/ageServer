package playfab

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/server/internal/models/athens/user"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

type GauntletMissionPool struct {
	Name     string
	Missions []user.ChallengeMission
}

type GauntletMissionPools []GauntletMissionPool

func ReadGauntletMissionPools() (gauntletMissionPools GauntletMissionPools) {
	if f, err := os.Open(filepath.Join(playfab.BaseDir, "public-production", "2", "gauntlet_mission_pools.json")); err == nil {
		defer func(f *os.File) {
			_ = f.Close()
		}(f)
		if err = json.NewDecoder(f).Decode(&gauntletMissionPools); err != nil {
			panic(err)
		}
		return gauntletMissionPools
	} else {
		panic(err)
	}
}
