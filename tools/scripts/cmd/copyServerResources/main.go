package main

import (
	"log"

	i "scripts/internal"

	e "github.com/luskaner/ageServer/common/executables"
)

func main() {
	module := e.Server
	src := i.ResourcePath(module)
	dst := i.BuildResourcePath(module)

	if err := i.SyncDir(src, dst, "certificates", "userData"); err != nil {
		log.Fatal(err)
	}
}
