package main

import (
	i "scripts/internal"

	e "github.com/luskaner/ageServer/common/executables"
)

func main() {
	module := e.Launcher
	i.CopyGameConfigs(module)
	i.CopyMainConfig(module)
}
