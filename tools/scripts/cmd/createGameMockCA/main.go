package main

import (
	"os"
	"path/filepath"
	"scripts/internal"
	"scripts/internal/constants"

	"github.com/luskaner/ageServer/common/game/cert"
)

func main() {
	g := os.Args[1]
	gamePath := filepath.Join(constants.BuildDir, "mock", "game", g)
	if ok, caCert := cert.NewCA(g, gamePath); ok {
		originalPath := caCert.OriginalPath()
		internal.MkdirP(filepath.Dir(originalPath))
		if _, err := os.Stat(originalPath); err != nil {
			if f, err := os.Create(originalPath); err != nil {
				panic(err)
			} else {
				_ = f.Close()
			}
		}
	}
}
