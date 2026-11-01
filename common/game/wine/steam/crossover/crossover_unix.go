//go:build !windows

package crossover

import (
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common/game"
)

const prefixEnvVar = "$CX_BOTTLE"

func Prefix(gameId string) string {
	var prefix string
	prefix = os.ExpandEnv(prefixEnvVar)
	if prefix == "" {
		prefix = defaultBottleName(gameId)
	}
	for _, baseDir := range baseDirs {
		if f, err := os.Stat(filepath.Join(os.ExpandEnv(baseDir), prefix)); err == nil && f.IsDir() {
			return prefix
		}
	}
	return ""
}

func baseDefaultBottleName(gameId string) (name string) {
	switch gameId {
	case game.AoE1:
		name = "Age of Empires Definitive Edition"
	case game.AoE2:
		name = "Age of Empires II Definitive Edition"
	case game.AoE3:
		name = "Age of Empires III Definitive Edition"
	case game.AoE4:
		name = "Age of Empires IV Anniversary Edition"
	case game.AoM:
		name = "Age of Mythology Retold"
	}
	return
}
