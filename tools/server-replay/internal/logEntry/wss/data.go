package wss

import (
	"encoding/json"
	"log"
	"net"

	"github.com/gorilla/websocket"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
	"github.com/luskaner/ageServer/server-replay/internal/logEntry"
)

type Data struct {
	Websocket[wss.Data]
	messageResponse
	response bool
}

// dataResponseMatches compares the LOGGED expected response against the
// actual one, tolerating volatile fields (dates, IPs) like CompareJSON.
//
// Regression: this used to compare the actual response against itself
// (d.body is messageResponse.body), so every WSS response reported OK.
func dataResponseMatches(expected []byte, actual []byte) bool {
	var from any
	if err := json.Unmarshal(expected, &from); err != nil {
		log.Printf("Error unmarshalling expected response: %s", err)
		return false
	}
	var to any
	if err := json.Unmarshal(actual, &to); err != nil {
		log.Printf("Error unmarshalling actual response: %s", err)
		return false
	}
	return logEntry.CompareJSON(from, to)
}

func (d *Data) CheckResponse() {
	if d.response {
		if d.messageResponse.err != nil {
			log.Println(d.messageResponse.err)
			return
		}
		if d.messageResponse.messageType != websocket.TextMessage {
			log.Printf("Not a text message")
			return
		}
		if len(d.data.Body.Body) == 0 {
			if !logEntry.SameBody(d.data.BodyHash.BodyHash, d.messageResponse.body) {
				log.Println("Body hash does not match")
				return
			}
		} else {
			if !dataResponseMatches(d.data.Body.Body, d.messageResponse.body) {
				return
			}
		}
	}
	log.Println("OK")
}

func (d *Data) Replay(_ net.IP) {
	if conn := d.conn(); conn == nil {
		panic("Cannot handle data: missing websocket connection")
	} else if d.sender() {
		if err := conn.WriteMessage(websocket.TextMessage, d.data.Body.Body); err != nil {
			log.Println(err)
		}
	} else {
		d.messageResponse.messageType, d.messageResponse.body, d.messageResponse.err = conn.ReadMessage()
		d.response = true
	}
}

func NewWebsocketData(base wss.Read, data *wss.Data) *Data {
	return &Data{
		Websocket: Websocket[wss.Data]{
			base,
			*data,
		},
	}
}
