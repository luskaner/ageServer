package Automatch2

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

// Polling handles POST /game/automatch2/polling for every title that uses the
// automatch2 engine. The per-title differences live in the queue catalog and
// in the positional response encoder, not in separate handlers.
func Polling(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	var req models.AutomatchPollingRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("automatch2: could not bind polling request: %v\n", err)
		i.JSON(&w, models.AutomatchUnmatchedResponse(g.Title(), 0, nil))
		return
	}
	sess := models.SessionOrPanic(r)
	response, notifications := g.Matchmaking().Poll(sess, &req, r)
	i.JSON(&w, response)
	wss.SendMatchNotifications(notifications)
}

// PollingAoE2 is kept as the explicit AoE2 entry point for the router and the
// tests; it shares the engine with Polling.
func PollingAoE2(w http.ResponseWriter, r *http.Request) {
	Polling(w, r)
}
