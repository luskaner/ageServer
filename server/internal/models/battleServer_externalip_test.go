package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/server/internal"
)

func withExternalLookup(t *testing.T, url string, client *http.Client) {
	t.Helper()
	origClient := externalIPClient
	origURL := externalIPURL
	origInternet := internal.CanUseInternet
	origPublic := publicIp
	origSubnets := localSubnets
	t.Cleanup(func() {
		externalIPClient = origClient
		externalIPURL = origURL
		internal.CanUseInternet = origInternet
		publicIp = origPublic
		localSubnets = origSubnets
	})
	externalIPURL = url
	externalIPClient = client
	internal.CanUseInternet = true
	publicIp = ""
	localSubnets = nil
}

// Regression: the public address lookup used http.DefaultClient, which has no
// timeout, followed by an unbounded io.ReadAll. It runs during startup before
// the server binds the port the launcher polls to decide it came up, and the
// launcher gives that a handful of seconds before killing the server it just
// started. A slow or half-open path was enough to get the server killed on the
// very machine that made it slow.
func TestExternalIPLookupIsBounded(t *testing.T) {
	// The handler answers with headers and then stalls the body. Release it
	// before Close, otherwise httptest.Server.Close blocks on the outstanding
	// request and the test hangs for a different reason.
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release
	}))
	defer slow.Close()
	defer close(release)

	withExternalLookup(t, slow.URL, &http.Client{Timeout: 200 * time.Millisecond})

	done := make(chan struct{})
	go func() {
		defer close(done)
		CacheNetworkInterfaces("auto")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("CacheNetworkInterfaces did not return: the lookup is unbounded")
	}
	if publicIp != "" {
		t.Errorf("publicIp = %q, want empty when the lookup times out", publicIp)
	}
}

// A well behaved endpoint must still work, so the timeout did not break the
// feature.
func TestExternalIPLookupSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("203.0.113.7"))
	}))
	defer srv.Close()

	withExternalLookup(t, srv.URL, &http.Client{Timeout: externalIPTimeout})
	CacheNetworkInterfaces("auto")
	if publicIp != "203.0.113.7" {
		t.Errorf("publicIp = %q, want %q", publicIp, "203.0.113.7")
	}
}

// A misbehaving endpoint must not be read for as long as the timeout allows.
func TestExternalIPBodyIsCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("9", 4096)))
	}))
	defer srv.Close()

	withExternalLookup(t, srv.URL, &http.Client{Timeout: externalIPTimeout})
	CacheNetworkInterfaces("auto")
	if publicIp != "" {
		t.Errorf("publicIp = %q, want empty: 4096 bytes is not an address", publicIp)
	}
}

func TestExternalIPTimeoutIsShorterThanTheLaunchersBudget(t *testing.T) {
	// The launcher polls the server's HTTPS endpoint for
	// interfaces * (LatencyMeasurementCount+1) seconds and then kills it. This
	// lookup has to fit comfortably inside that or a slow machine gets the
	// server it just started killed again.
	if externalIPTimeout <= 0 {
		t.Fatal("externalIPTimeout must be positive")
	}
	if externalIPTimeout > 8*time.Second {
		t.Errorf("externalIPTimeout = %v, too long to be safe before the server binds its port", externalIPTimeout)
	}
}
