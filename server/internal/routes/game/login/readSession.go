package login

import (
	"encoding/json"
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func returnData(w http.ResponseWriter, messageId uint, messages []i.A) {
	// Marshal the id too: concatenating it as a raw number keeps the wire
	// format but writes only escaped value data to the response.
	id, _ := json.Marshal(messageId)
	j, _ := json.Marshal(i.A{messages})
	var body []byte
	body = append(body, id...)
	body = append(body, ',')
	body = append(body, j...)
	i.RawJSON(&w, body)
}

func returnError(w http.ResponseWriter) {
	returnData(w, 0, []i.A{})
}

type readSessionRequest struct {
	Ack uint `schema:"ack"`
}

func ReadSession(w http.ResponseWriter, r *http.Request) {
	var req readSessionRequest
	err := i.Bind(r, &req)
	if err != nil {
		returnError(w)
		return
	}
	sess := models.SessionOrPanic(r)
	messageId, messages := sess.WaitForMessages(req.Ack)
	returnData(w, messageId, messages)
}
