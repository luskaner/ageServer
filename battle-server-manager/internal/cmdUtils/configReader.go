package cmdUtils

import (
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/battleServer"
)

func ExistingServers(gameId string) (err error, names mapset.Set[string], regions mapset.Set[string]) {
	var configs []battleServer.Config
	configs, err = battleServer.Configs(gameId, true, false)
	if err != nil {
		return
	}
	names = mapset.NewThreadUnsafeSetWithSize[string](len(configs))
	regions = mapset.NewThreadUnsafeSetWithSize[string](len(configs))
	for _, config := range configs {
		names.Add(strings.ToLower(config.Name))
		regions.Add(strings.ToLower(config.Region))
	}
	return
}
