package wss

import (
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
)

func TestWebsocketUptimeAndString(t *testing.T) {
	now := 100 * time.Millisecond
	w := &Websocket[wss.Connection]{
		base: wss.Read{
			BaseRead: wss.BaseRead{
				Uptime: serverCommunication.Uptime{Uptime: now},
				Sender: serverCommunication.Sender{Sender: "sender1"},
				Receiver: "127.0.0.1:443",
				Subtype: "Connection",
			},
		},
	}
	if w.Uptime() != now {
		t.Fatalf("uptime %v", w.Uptime())
	}
	s := w.String()
	if s == "" {
		t.Fatal("string empty")
	}
	// Test sender true (port 443)
	if !w.sender() {
		t.Fatal("should be sender")
	}
	// Test sender false
	w2 := &Websocket[wss.Connection]{
		base: wss.Read{
			BaseRead: wss.BaseRead{
				Receiver: "127.0.0.1:80",
			},
		},
	}
	if w2.sender() {
		t.Fatal("should not be sender")
	}
	// Test malformed receiver
	w3 := &Websocket[wss.Connection]{
		base: wss.Read{
			BaseRead: wss.BaseRead{
				Receiver: "bad",
			},
		},
	}
	if w3.sender() {
		t.Fatal("malformed should be false")
	}
}

func TestWebsocketDataTypes(t *testing.T) {
	// Test with Data
	w := &Websocket[wss.Data]{
		base: wss.Read{
			BaseRead: wss.BaseRead{
				Uptime: serverCommunication.Uptime{Uptime: 10 * time.Millisecond},
				Subtype: "Data",
			},
		},
		data: wss.Data{},
	}
	if w.Uptime() != 10*time.Millisecond {
		t.Fatal("uptime")
	}
	_ = w.String()
}
