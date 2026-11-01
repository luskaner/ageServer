package Automatch2

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

func Stoppolling(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement for matchmaking
	i.JSON(&w, i.A{0})
}

func StopPolling(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	var req models.AutomatchStopPollingRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("automatch2: could not bind stop polling request: %v\n", err)
		i.JSON(&w, i.A{0})
		return
	}
	i.JSON(&w, g.Matchmaking().StopPolling(models.SessionOrPanic(r), &req))
}
