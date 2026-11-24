package main

import (
	"log"

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
	// puts it and the server looks for it.
	i.CopyGameConfigsTo(module, p.ConfigDir)
}
