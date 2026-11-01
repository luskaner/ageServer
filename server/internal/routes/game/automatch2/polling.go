package Automatch2

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

func Polling(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement for matchmaking
	i.JSON(&w, i.A{
		2,           // resultCode: 2 = error (not implemented)
		i.A{},       // match
		-1,          // matchId
		nil,         // host
		-1,          // state
		"",          // description
		-1,          // maxPlayers
		-1,          // currentPlayers
		-1,          // slotCount
		"0",         // isRanked
		"0",         // isTournament
		"-1",        // mapId
		-1,          // matchType
		"authtoken", // auth token
		"0.0.0.0",   // server IP
		0,           // port
		0,           // gamePort
		0,           // relayPort
		"",          // relayHost
		i.A{},       // slots
		nil,         // matchData
		0,           // ping
	})
}

func PollingAoE2(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	var req models.AutomatchPollingRequest
	if err := i.Bind(r, &req); err != nil {
		logger.Printf("automatch2: could not bind polling request: %v\n", err)
		i.JSON(&w, models.AutomatchUnmatchedResponse(0, nil))
		return
	}
	sess := models.SessionOrPanic(r)
	response, notifications := g.Matchmaking().Poll(sess, &req, r)
	i.JSON(&w, response)
	wss.SendMatchNotifications(notifications)
}
