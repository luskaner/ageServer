package wss

import (
	"log"
	"net"

	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
)

type Disconnection struct {
	Websocket[wss.Disconnection]
}

func (c *Disconnection) CheckResponse() {}

func (c *Disconnection) Replay(_ net.IP) {
	if c.sender() {
		if conn := c.conn(); conn == nil {
			log.Println("Cannot handle disconnection: missing websocket connection")
		} else {
			_ = conn.Close()
		}
	} else if conn := c.conn(); conn != nil {
		log.Println("Disconnection: websocket connection should be missing")
	}
}

func NewWebsocketDisconnection(base wss.Read, data *wss.Disconnection) *Disconnection {
	return &Disconnection{
		Websocket[wss.Disconnection]{
			base,
			*data,
		},
	}
}
