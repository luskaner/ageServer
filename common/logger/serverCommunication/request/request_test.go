package request

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
)

func TestNewWrite(t *testing.T) {
	u, _ := url.Parse("https://example.com/test")
	in := In{
		Base: Base{
			Body:    serverCommunication.Body{Body: []byte("hello")},
			Headers: http.Header{"Content-Type": {"application/json"}},
		},
		Uptime: serverCommunication.Uptime{Uptime: 5 * time.Second},
		Sender: serverCommunication.Sender{Sender: "127.0.0.1:1234"},
		Url:    u,
		Method: "POST",
	}
	out := Out{
		Base: Base{
			Body:    serverCommunication.Body{Body: []byte("response")},
			Headers: http.Header{"X-Test": {"val"}},
		},
		BodyHash:   serverCommunication.BodyHash{BodyHash: [64]byte{1, 2, 3}},
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
	}
	read := Read{In: in, Out: out}

	w := NewWrite(read)

	if w.Type != serverCommunication.MessageRequest {
		t.Errorf("Type = %q, want %q", w.Type, serverCommunication.MessageRequest)
	}
	if string(w.In.Body.Body) != "hello" {
		t.Errorf("In.Body = %q, want %q", string(w.In.Body.Body), "hello")
	}
	if w.In.Method != "POST" {
		t.Errorf("In.Method = %q, want %q", w.In.Method, "POST")
	}
	if w.Out.StatusCode != 200 {
		t.Errorf("Out.StatusCode = %d, want 200", w.Out.StatusCode)
	}
	if w.Out.Latency != 10*time.Millisecond {
		t.Errorf("Out.Latency = %v, want %v", w.Out.Latency, 10*time.Millisecond)
	}
}
