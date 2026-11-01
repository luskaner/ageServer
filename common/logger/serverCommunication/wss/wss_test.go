package wss

import (
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
)

func TestNewWriteConnection(t *testing.T) {
	w := NewWrite(
		Connection{},
		serverCommunication.Uptime{Uptime: 1 * time.Second},
		serverCommunication.Sender{Sender: "192.168.1.1:5000"},
		"server:443",
	)
	if w.Type != serverCommunication.MessageWSS {
		t.Errorf("Type = %q, want %q", w.Type, serverCommunication.MessageWSS)
	}
	if w.Subtype != "Connection" {
		t.Errorf("Subtype = %q, want %q", w.Subtype, "Connection")
	}
	if w.Sender.Sender != "192.168.1.1:5000" {
		t.Errorf("Sender = %q, want %q", w.Sender.Sender, "192.168.1.1:5000")
	}
	if w.Receiver != "server:443" {
		t.Errorf("Receiver = %q, want %q", w.Receiver, "server:443")
	}
}

func TestNewWriteDisconnection(t *testing.T) {
	w := NewWrite(
		Disconnection{},
		serverCommunication.Uptime{Uptime: 2 * time.Second},
		serverCommunication.Sender{Sender: "client"},
		"server",
	)
	if w.Subtype != "Disconnection" {
		t.Errorf("Subtype = %q, want %q", w.Subtype, "Disconnection")
	}
}

func TestNewWriteData(t *testing.T) {
	d := Data{
		Body:     serverCommunication.Body{Body: []byte("payload")},
		BodyHash: serverCommunication.BodyHash{BodyHash: [64]byte{10}},
	}
	w := NewWrite(
		d,
		serverCommunication.Uptime{Uptime: 3 * time.Second},
		serverCommunication.Sender{Sender: "a"},
		"b",
	)
	if w.Subtype != "Data" {
		t.Errorf("Subtype = %q, want %q", w.Subtype, "Data")
	}
	if w.Data.Body.Body == nil {
		t.Error("Data.Body.Body should not be nil")
	}
}

func TestNewWriteControl(t *testing.T) {
	c := Control{
		Data:        Data{Body: serverCommunication.Body{Body: []byte{}}},
		MessageType: 42,
	}
	w := NewWrite(
		c,
		serverCommunication.Uptime{Uptime: 0},
		serverCommunication.Sender{Sender: "s"},
		"r",
	)
	if w.Subtype != "Control" {
		t.Errorf("Subtype = %q, want %q", w.Subtype, "Control")
	}
	if w.Data.MessageType != 42 {
		t.Errorf("Control.MessageType = %d, want 42", w.Data.MessageType)
	}
}
