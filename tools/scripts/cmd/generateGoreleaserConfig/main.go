package main

import (
	"os"

	"scripts/internal/goreleaser"
)

func main() {
	// Args: [0]=program, [1]=GOROOT_LEGACY (optional), [2]="current" (optional)
	// When run from Taskfile: GOROOT_LEGACY is passed as first arg
	// When run manually: "current" might be first arg
	var currentOnly bool
	if len(os.Args) > 1 {
		if os.Args[1] == "current" {
			currentOnly = true
		} else if len(os.Args) > 2 && os.Args[2] == "current" {
			currentOnly = true
		}
	}
	if currentOnly {
		if err := goreleaser.GenerateCurrent(); err != nil {
			panic(err)
		}
		return
	}
	if err := goreleaser.Generate(); err != nil {
		panic(err)
	}
}
