package request

import (
	"net/http"
	"net/url"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
)

type Base struct {
	serverCommunication.Body
	Headers http.Header
}

type In struct {
	Base
	serverCommunication.Uptime
	serverCommunication.Sender
	Url    *url.URL
	Method string
}

type Out struct {
	Base
	serverCommunication.BodyHash
	StatusCode int
	Latency    time.Duration
}

type Read struct {
	In  `json:"in"`
	Out `json:"out"`
}

type Write struct {
	Read
	serverCommunication.MessageType
}

func NewWrite(read Read) Write {
	return Write{
		read,
		serverCommunication.MessageType{Type: serverCommunication.MessageRequest},
	}
}
