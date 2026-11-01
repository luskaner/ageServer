package wss

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
	clientWss "github.com/luskaner/ageServer/server-replay/internal/client/wss"
)

type messageResponse struct {
	messageType int
	body        []byte
	err         error
}

type Websocket[D wss.MessageType] struct {
	base wss.Read
	data D
}

func (w *Websocket[D]) Uptime() time.Duration {
	return w.base.Uptime.Uptime
}

func (w *Websocket[D]) String() string {
	str := ""
	if w.sender() {
		str += "->"
	} else {
		str += "<-"
	}
	return str + " Websocket " + w.base.Subtype
}

func (w *Websocket[D]) sender() bool {
	if _, port, err := net.SplitHostPort(w.base.Receiver); err == nil {
		return port == "443"
	}
	// A malformed receiver address cannot be attributed to the server side.
	return false
}

func (w *Websocket[D]) conn() *websocket.Conn {
	var id string
	if w.sender() {
		id = w.base.Sender.Sender
	} else {
		id = w.base.Receiver
	}
	return clientWss.Get(id)
}
