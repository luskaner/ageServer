package userData

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
)

func (u *Path) profileFolder() string {
	var p string
	switch u.GameId() {
	case game.AoE1, game.AoE4:
		p = "Users"
	}
	return p
}

func (u *Path) Profiles() (err error, profiles mapset.Set[Data]) {
	var entries []os.DirEntry
	baseDir := filepath.Join(u.String(), u.profileFolder())
	entries, err = os.ReadDir(baseDir)
	if err != nil {
		return
	}
	profiles = mapset.NewThreadUnsafeSet[Data]()
	for _, entry := range entries {
		if entry.IsDir() {
			t, ext := typ(entry.Name())
			nameForParse := entry.Name()
			if t != TypeActive {
				nameForParse = strings.TrimSuffix(nameForParse, ext)
			}
			if u.gameId != game.AoE1 {
				if _, localErr := strconv.ParseUint(nameForParse, 10, 64); localErr != nil {
					continue
				}
			}
			profiles.Add(Data{t, filepath.Join(baseDir, entry.Name())})
		}
	}
	return
}
