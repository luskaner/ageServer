package cert

import (
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
)

type CA struct {
	gamePath string
}

func NewCA(gameId string, gamePath string) (ok bool, ca CA) {
	if !HasCA(gameId) {
		return
	}
	if gameId == game.AoE2 {
		gamePath = filepath.Join(gamePath, "certificates")
	}
	ok = true
	ca = CA{gamePath: gamePath}
	return
}

func HasCA(gameId string) bool {
	return gameId != game.AoE1 && gameId != game.AoE4
}

func (c CA) name() string {
	return common.CACert
}

func (c CA) OriginalPath() string {
	return filepath.Join(c.gamePath, c.name())
}

func (c CA) TmpPath() string {
	return filepath.Join(c.gamePath, c.name()+".lan")
}

func (c CA) BackupPath() string {
	return filepath.Join(c.gamePath, c.name()+".bak")
}
