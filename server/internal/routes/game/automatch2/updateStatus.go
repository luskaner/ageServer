package Automatch2

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

func UpdateStatus(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	var req models.AutomatchUpdateStatusRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("automatch2: could not bind update status request: %v\n", err)
		i.JSON(&w, i.A{0, 0})
		return
	}
	i.JSON(&w, g.Matchmaking().UpdateStatus(models.SessionOrPanic(r), &req))
}
