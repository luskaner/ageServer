package playfab

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

type ColumnConfig struct {
	MissionPool string
}

type LabyrinthConfig struct {
	ForGauntletDifficulties []string
	ColumnConfigs           []ColumnConfig
	BossMissionPool         string
}

type Rewards struct {
	ExcludeFromRegularRewards []Blessings
	PreferredFinalRewards     []any
}

type Gauntlet struct {
	LabyrinthConfigs []LabyrinthConfig
	Rewards          Rewards
}

func ReadGauntlet() (gauntlet Gauntlet) {
	if f, err := os.Open(filepath.Join(playfab.BaseDir, "public-production", "2", "gauntlet.json")); err == nil {
		defer func(f *os.File) {
			_ = f.Close()
		}(f)
		if err = json.NewDecoder(f).Decode(&gauntlet); err != nil {
			panic(err)
		}
		return gauntlet
	} else {
		panic(err)
	}
}
