package wss

import (
	"testing"

	"github.com/luskaner/ageServer/common/logger/serverCommunication/wss"
)

func mkData(expected string, actual string) *Data {
	d := &Data{}
	d.data.Body.Body = []byte(expected)
	d.messageResponse.body = []byte(actual)
	return d
}

// Regression: CheckResponse compared the actual response against itself
// (d.body IS messageResponse.body), so every WSS entry reported OK regardless
// of what the server returned.
func TestDataResponseMatchesAgainstLoggedExpected(t *testing.T) {
	if !dataResponseMatches([]byte(`{"a":1}`), []byte(`{"a":  1}`)) {
		t.Error("semantically equal JSON must match")
	}
	if dataResponseMatches([]byte(`{"a":1}`), []byte(`{"a":2}`)) {
		t.Fatal("different values were accepted (tautological comparison)")
	}
	if !dataResponseMatches([]byte(`{"Updated":"2026-08-23T10:00:00.000Z"}`), []byte(`{"Updated":"2025-01-01T00:00:00.000Z"}`)) {
		t.Fatal("dates are volatile and must be tolerated")
	}
	if dataResponseMatches([]byte(`not-json`), []byte(`{"a":1}`)) {
		t.Fatal("unparsable expected body must not match")
	}
}

func TestSenderDetection(t *testing.T) {
	serverSide := Websocket[wss.Data]{}
	serverSide.base.Receiver = "192.168.1.5:443"
	if !serverSide.sender() {
		t.Fatal("receiver on port 443 means server->client (sender)")
	}

	clientSide := Websocket[wss.Data]{}
	clientSide.base.Receiver = "192.168.1.5:51234"
	if clientSide.sender() {
		t.Fatal("ephemeral receiver port means client->server (not sender)")
	}
}

// Regression: sender() panicked when Receiver had no port.
func TestSenderWithMalformedReceiverDoesNotPanic(t *testing.T) {
	w := Websocket[wss.Data]{}
	w.base.Receiver = "no-port-here"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	_ = w.sender()
}
