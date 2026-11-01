package http

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication/request"
)

func newFixture(t *testing.T, host string, port string) *Request {
	t.Helper()
	u := &url.URL{Scheme: "https", Host: host + ":" + port, Path: "/some/endpoint"}
	return NewRequest(request.Read{
		In: request.In{
			Url:    u,
			Method: "GET",
		},
	})
}

// Regression: Replay panicked on transport errors (connection refused), killing
// the whole replay run for one failed request.
func TestReplayTransportFailureDoesNotPanic(t *testing.T) {
	r := newFixture(t, "127.0.0.1", "1") // port 1: connection refused

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	r.Replay(nil)

	if r.response.statusCode != 0 {
		t.Fatalf("statusCode = %d, want 0 on failure", r.response.statusCode)
	}
}

// Regression: an unparsable stored URL/method used to panic via
// http.NewRequest.
func TestReplayInvalidStoredRequestDoesNotPanic(t *testing.T) {
	r := NewRequest(request.Read{
		In: request.In{
			Url:    &url.URL{Scheme: "https", Host: "127.0.0.1:443", Path: "/x"},
			Method: "BAD METHOD", // contains a space -> invalid
		},
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	r.Replay(nil)

	if !r.ignored {
		t.Fatal("unbuildable request must be marked ignored")
	}
}

func TestUptimeExposesLoggedValue(t *testing.T) {
	r := NewRequest(request.Read{})
	r.data.Uptime.Uptime = 1234 * time.Millisecond
	if got := r.Uptime(); got != 1234*time.Millisecond {
		t.Fatalf("uptime = %v", got)
	}
}

func TestCookiesFromSetCookieHeader(t *testing.T) {
	h := http.Header{}
	h.Set("Set-Cookie", "session=abc123; Path=/; Max-Age=3600")
	cookies := cookies(h)
	// http.ParseCookie splits on ';', so all three segments become cookies
	if len(cookies) == 0 {
		t.Fatal("cookies = 0, want > 0")
	}
	// The first cookie must be the real one
	if cookies[0].Name != "session" || cookies[0].Value != "abc123" {
		t.Errorf("cookie = %s=%s", cookies[0].Name, cookies[0].Value)
	}
	// MaxAge must be stripped from all cookies
	for _, c := range cookies {
		if c.MaxAge != 0 {
			t.Errorf("MaxAge = %d for %s, want 0", c.MaxAge, c.Name)
		}
	}
}

func TestCookiesEmptyHeader(t *testing.T) {
	h := http.Header{}
	cookies := cookies(h)
	if len(cookies) != 0 {
		t.Fatalf("cookies = %d, want 0", len(cookies))
	}
}

func TestCookiesInvalidHeader(t *testing.T) {
	h := http.Header{}
	h.Set("Set-Cookie", "%%%invalid")
	cookies := cookies(h)
	if len(cookies) != 0 {
		t.Fatalf("cookies = %d, want 0 for invalid cookie", len(cookies))
	}
}

func TestDelHeaderRemovesFromBoth(t *testing.T) {
	h1 := http.Header{"X-Auth": []string{"a"}, "Keep": []string{"k"}}
	h2 := http.Header{"X-Auth": []string{"b"}, "Keep": []string{"k"}}
	delHeader(&h1, &h2, "X-Auth")
	if _, ok := h1["X-Auth"]; ok {
		t.Error("h1 still has X-Auth")
	}
	if _, ok := h2["X-Auth"]; ok {
		t.Error("h2 still has X-Auth")
	}
	if _, ok := h1["Keep"]; !ok {
		t.Error("h1 lost Keep")
	}
}

func TestDelHeaderMultipleValues(t *testing.T) {
	h1 := http.Header{"A": []string{"1"}, "B": []string{"2"}, "C": []string{"3"}}
	h2 := http.Header{"A": []string{"1"}, "B": []string{"2"}, "C": []string{"3"}}
	delHeader(&h1, &h2, "A", "C")
	if _, ok := h1["A"]; ok {
		t.Error("h1 still has A")
	}
	if _, ok := h1["C"]; ok {
		t.Error("h1 still has C")
	}
	if _, ok := h1["B"]; !ok {
		t.Error("h1 lost B")
	}
}

func TestRequestMethodAndURL(t *testing.T) {
	u := &url.URL{Scheme: "https", Host: "example.com:443", Path: "/api/test"}
	r := NewRequest(request.Read{
		In: request.In{
			Url:    u,
			Method: "POST",
		},
	})
	got := r.String()
	want := "POST https://example.com:443/api/test"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
