package router

import (
	"net/http"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/routes/apiAgeOfEmpires/textmoderation"
)

type ApiAgeOfEmpires struct {
	Proxy
	apiName string
}

func (a *ApiAgeOfEmpires) Name() string {
	return a.apiName
}

func (a *ApiAgeOfEmpires) Initialize(_ string) bool {
	return true
}

func (a *ApiAgeOfEmpires) InitializeRoutes(gameId string, next http.Handler) http.Handler {
	a.Proxy = NewProxy(a.Name(), func(_ string, _ http.Handler) http.Handler {
		a.group.HandleFunc("POST", "/textmoderation", textmoderation.TextModeration)
		return a.group.mux
	})
	// The embedded library is mounted once, around the whole host handler, so
	// every tree it owns (mods and MatchContent) is handled before the proxy
	// fallback instead of being registered prefix by prefix.
	return withAgeAPIServer(a.Proxy.InitializeRoutes(gameId, next), gameId, a.Proxy.Upstream(), true)
}

func NewApiAgeOfEmpires() *ApiAgeOfEmpires {
	return &ApiAgeOfEmpires{
		apiName: common.ApiAgeOfEmpires,
	}
}

type Aoe4ApiAgeOfEmpires struct {
	ApiAgeOfEmpires
}

func (a *Aoe4ApiAgeOfEmpires) Initialize(gameId string) bool {
	return gameId == game.AoE4
}

func NewAoe4ApiAgeOfEmpires() *Aoe4ApiAgeOfEmpires {
	return &Aoe4ApiAgeOfEmpires{
		ApiAgeOfEmpires{
			apiName: common.Aoe4ApiAgeOfEmpires,
		},
	}
}
