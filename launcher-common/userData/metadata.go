package userData

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/game"
)

func (u *Path) metadataFolder() string {
	var p string
	switch u.GameId() {
	case game.AoE2:
		p = "metadata"
	case game.AoE3:
		p = filepath.Join("Common", "RLink")
	case game.AoE4:
		p = "network"
	case game.AoM:
		p = filepath.Join("temp", "RLink")
	}
	return p
}

func (u *Path) Metadatas() (err error, metadatas mapset.Set[Data]) {
	p := filepath.Join(u.String(), u.metadataFolder())
	if _, err = os.Stat(p); errors.Is(err, fs.ErrNotExist) {
		oldParent := p
		for parent := filepath.Dir(p); parent != oldParent; parent = filepath.Dir(parent) {
			var f os.FileInfo
			if f, err = os.Stat(parent); err == nil {
				if err = os.MkdirAll(p, f.Mode().Perm()); err != nil {
					return
				}
				break
			}
			oldParent = parent
		}
	}
	err = nil
	metadatas = mapset.NewThreadUnsafeSet[Data]()
	if p != "" {
		allSuffixes := maps.Clone(suffixToType)
		allSuffixes[""] = TypeActive
		for _, t := range allSuffixes {
			if ok, transformedPath := TransformPath(p, TypeActive, t); ok {
				if f, localErr := os.Stat(transformedPath); localErr == nil && f.IsDir() {
					metadatas.Add(Data{t, transformedPath})
				}
			}
		}
	}
	return
}
