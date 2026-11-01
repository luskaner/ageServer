package router

import (
	"net/http"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/routes/apiAgeOfEmpires/textmoderation"
	"github.com/luskaner/ageServer/server/internal/routes/matchcontent"
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
		a.group.HandleFunc("POST", "/api/MatchContent/", matchcontent.Upload)
		a.group.HandleFunc("GET", "/api/MatchContent/{matchId}", matchcontent.Download)
		a.group.HandleFunc("GET", "/api/MatchContent/{matchId}/file/{hash}", matchcontent.DownloadFile)
		return a.group.mux
	})
	return a.Proxy.InitializeRoutes(gameId, next)
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
