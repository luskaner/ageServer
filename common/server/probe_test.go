package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// ProbeApiServer is what decides whether a configured API server is usable, so
// what it asks and what it reports are both part of the contract.
func TestProbeApiServerReturnsTheStatus(t *testing.T) {
	oldPort := serverPort
	t.Cleanup(func() { serverPort = oldPort })

	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/"+common.ApiTestPath {
				t.Errorf("probe asked for %q, want /%s", r.URL.Path, common.ApiTestPath)
			}
			if got := r.Header.Get("User-Agent"); got != common.UserAgent() {
				t.Errorf("probe User-Agent = %q, want %q", got, common.UserAgent())
			}
			w.WriteHeader(status)
		}))
		parsed, err := url.Parse(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(parsed.Host)
		if err != nil {
			t.Fatal(err)
		}
		serverPort = port
		got, probeErr := ProbeApiServer("127.0.0.1", true, nil)
		server.Close()
		if probeErr != nil {
			t.Fatalf("ProbeApiServer with status %d failed: %v", status, probeErr)
		}
		if got != status {
			t.Fatalf("ProbeApiServer = %d, want %d", got, status)
		}
	}
}

func TestProbeApiServerReportsAnUnreachableServer(t *testing.T) {
	oldPort := serverPort
	t.Cleanup(func() { serverPort = oldPort })
	// A port nothing listens on: the probe has to report the failure instead
	// of a status, because there is no answer to report.
	serverPort = "1"
	if _, err := ProbeApiServer("127.0.0.1", true, nil); err == nil {
		t.Fatal("ProbeApiServer reported success for a server that is not listening")
	}
}
