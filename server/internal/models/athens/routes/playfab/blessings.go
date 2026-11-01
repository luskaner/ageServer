package playfab

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

type Blessings struct {
	EffectName    string
	KnownRarities []int
}

type BlessingsJson struct {
	KnownBlessings []Blessings
}

func ReadBlessings() (blessings []Blessings) {
	if f, err := os.Open(filepath.Join(playfab.BaseDir, "public-production", "2", "known_blessings.json")); err == nil {
		defer func(f *os.File) {
			_ = f.Close()
		}(f)
		var data []byte
		data, err = io.ReadAll(f)
		if err == nil {
			var blessingsJson BlessingsJson
			if err = json.Unmarshal(data, &blessingsJson); err == nil {
				blessings = blessingsJson.KnownBlessings
			}
		}
	}
	return
}
