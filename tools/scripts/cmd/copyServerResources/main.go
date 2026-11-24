package main

import (
	"fmt"
	"log"
	"path/filepath"

	i "scripts/internal"

	e "github.com/luskaner/ageServer/common/executables"
	p "github.com/luskaner/ageServer/common/paths"
)

func main() {
	module := e.Server
	src := i.ResourcePath(module)
	dst := i.BuildResourcePath(module)

	if err := i.SyncDir(src, dst, "certificates", "userData"); err != nil {
		log.Fatal(err)
	}
	// One document per game, next to config.toml, exactly where the release
	// puts it and the server looks for it. The template lives in config/ for server.
	templateSrc := filepath.Join(src, p.ConfigDir, fmt.Sprintf("config.game.toml"))
	i.CopyGameConfigsToWithTemplate(module, p.ConfigDir, templateSrc)
}
