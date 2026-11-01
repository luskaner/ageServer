package Event

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/routes/playfab/Client/shared"
)

type writeTelemetryEventsResponseStruct struct {
	AssignedEventIds []any
}

func WriteTelemetryEvents(w http.ResponseWriter, _ *http.Request) {
	shared.RespondOK(&w, writeTelemetryEventsResponseStruct{AssignedEventIds: []any{}})
}
