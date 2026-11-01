package wss

import (
	"encoding/json"
	"reflect"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
)

type BaseRead struct {
	serverCommunication.Uptime
	serverCommunication.Sender
	Receiver string
	Subtype  string
}

type Read struct {
	BaseRead
	Data json.RawMessage
}

type Write[T MessageType] struct {
	BaseRead
	serverCommunication.MessageType
	Data T
}

type MessageType interface {
	Connection | Disconnection | Control | Data
}

type Connection struct{}

type Disconnection struct{}

type Data struct {
	serverCommunication.Body
	serverCommunication.BodyHash
}

type Control struct {
	Data
	MessageType int
}

func NewWrite[T MessageType](data T, uptime serverCommunication.Uptime, sender serverCommunication.Sender, receiver string) Write[T] {
	return Write[T]{
		BaseRead{
			Uptime:   uptime,
			Sender:   sender,
			Receiver: receiver,
			Subtype:  reflect.TypeOf(data).Name(),
		},
		serverCommunication.MessageType{Type: serverCommunication.MessageWSS},
		data,
	}
}
