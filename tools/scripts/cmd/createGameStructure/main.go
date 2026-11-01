package main

import (
	i "scripts/internal"

	e "github.com/luskaner/ageServer/common/executables"
)

func main() {
	i.CopyGameConfigs(e.BattleServerManager)
}
