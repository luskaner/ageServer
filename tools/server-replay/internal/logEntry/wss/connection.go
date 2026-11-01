package wss

import (
	"net"
	"net/url"

	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
	clientWss "github.com/luskaner/ageServer/server-replay/internal/client/wss"
)

type Connection struct {
	Websocket[wss.Connection]
}

func (c *Connection) CheckResponse() {}

func (c *Connection) Replay(serverIP net.IP) {
	u := url.URL{
		Scheme: "wss",
		Host:   serverIP.String(),
		Path:   "/wss/",
	}
	if err := clientWss.New(c.base.Sender.Sender, u.String(), nil); err != nil {
		panic(err)
	}
}

func NewWebsocketConnection(base wss.Read, data *wss.Connection) *Connection {
	return &Connection{
		Websocket[wss.Connection]{
			base,
			*data,
		},
	}
}
