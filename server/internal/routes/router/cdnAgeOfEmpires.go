package router

import (
	"fmt"
	"net/http"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/routes/cdnAgeOfEmpires/aoe/serverStatus"
)

type CdnAgeOfEmpires struct {
	Proxy
}

func (c *CdnAgeOfEmpires) Name() string {
	return common.CdnAgeOfEmpires
}

func (c *CdnAgeOfEmpires) Initialize(_ string) bool {
	return true
}

func (c *CdnAgeOfEmpires) InitializeRoutes(gameId string, next http.Handler) http.Handler {
	c.Proxy = NewProxy(common.CdnAgeOfEmpires, func(gameId string, next http.Handler) http.Handler {
		var prefix string
		if gameId == game.AoM {
			prefix = "athens"
		} else {
			prefix = "rl"
		}
		aoeGroup := c.group.Subgroup("/aoe")
		aoeGroup.HandleFunc("GET", fmt.Sprintf("/%s-server-status.json", prefix), serverStatus.ServerStatus)
		return c.group.mux
	})
	return c.Proxy.InitializeRoutes(gameId, next)
}
