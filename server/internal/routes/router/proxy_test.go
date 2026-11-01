package router

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal"
)

// stubResolver implements common.Resolver without touching the network so the
// NewProxy internet gate can be tested deterministically.
type stubResolver struct {
	ip     string
	dnsErr error
	calls  atomic.Int32
}

func (s *stubResolver) HostToIPs(string) []net.IP { return nil }
func (s *stubResolver) IPToHosts(string) []string { return nil }
func (s *stubResolver) DirectHostToIP(host string) (string, error) {
	s.calls.Add(1)
	return s.ip, s.dnsErr
}
func (s *stubResolver) DialTCP(network, address string, timeout time.Duration) (net.Conn, error) {
	return nil, errors.New("offline")
}
func (s *stubResolver) NetInterfaces() ([]net.Interface, error) { return nil, nil }
func (s *stubResolver) RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error) {
	return nil, nil
}

func withCanUseInternet(t *testing.T, enabled bool) {
	t.Helper()
	old := internal.CanUseInternet
	internal.CanUseInternet = enabled
	t.Cleanup(func() { internal.CanUseInternet = old })
}

func withStubResolver(t *testing.T, s *stubResolver) {
	t.Helper()
	t.Cleanup(common.SetResolver(s))
}

func TestNewProxyNoInternet(t *testing.T) {
	withCanUseInternet(t, false)
	stub := &stubResolver{}
	withStubResolver(t, stub)

	p := NewProxy("cdn.ageofempires.com", nil)
	if p.proxy != nil {
		t.Fatal("proxy should not be created without internet")
	}
	if p.host != "cdn.ageofempires.com" {
		t.Fatalf("host = %q", p.host)
	}
	if got := p.Name(); got != "proxy cdn.ageofempires.com" {
		t.Fatalf("Name() = %q", got)
	}
	if calls := stub.calls.Load(); calls != 0 {
		t.Fatalf("DirectHostToIP called %d times without internet, want 0", calls)
	}
}

func TestNewProxyCheck(t *testing.T) {
	withCanUseInternet(t, false)
	p := NewProxy("cdn.ageofempires.com", nil)

	cases := []struct {
		host string
		want bool
	}{
		{"cdn.ageofempires.com", true},
		{"cdn.ageofempires.com:443", true},
		{"CDN.aGeOfEmPiReS.CoM", true},
		{"other.example.com", false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		if got := p.Check(r); got != tc.want {
			t.Errorf("Check(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestNewProxyOfflineOnlyLocalRoutes(t *testing.T) {
	withCanUseInternet(t, false)
	stub := &stubResolver{}
	withStubResolver(t, stub)

	var p Proxy
	p = NewProxy("cdn.ageofempires.com", func(_ string, _ http.Handler) http.Handler {
		p.group.HandleFunc("GET", "/local", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		return p.group.mux
	})
	h := p.InitializeRoutes("age1", nil)

	req := httptest.NewRequest("GET", "http://cdn.ageofempires.com/local", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("local route = %d, want 200", rr.Code)
	}

	req = httptest.NewRequest("GET", "http://cdn.ageofempires.com/unhandled", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unproxied path = %d, want 404 (no reverse proxy mounted)", rr.Code)
	}
}

func TestNewProxyWithInternet(t *testing.T) {
	withCanUseInternet(t, true)
	stub := &stubResolver{ip: "1.2.3.4"}
	withStubResolver(t, stub)

	p := NewProxy("cdn.ageofempires.com", nil)
	if p.proxy == nil {
		t.Fatal("proxy should be created with internet")
	}
	if calls := stub.calls.Load(); calls != 1 {
		t.Fatalf("DirectHostToIP called %d times, want 1", calls)
	}

	in := httptest.NewRequest("GET", "http://cdn.ageofempires.com/aoe/x", nil)
	pr := &httputil.ProxyRequest{In: in, Out: in.Clone(in.Context())}
	p.proxy.Rewrite(pr)
	if pr.Out.URL.Scheme != "https" {
		t.Errorf("rewrite scheme = %q, want https", pr.Out.URL.Scheme)
	}
	if pr.Out.URL.Host != "1.2.3.4" {
		t.Errorf("rewrite out host = %q, want resolved IP", pr.Out.URL.Host)
	}
	if pr.Out.Host != "cdn.ageofempires.com" {
		t.Errorf("rewrite Host header = %q, want original host", pr.Out.Host)
	}

	tp, ok := p.proxy.Transport.(*http.Transport)
	if !ok || tp.TLSClientConfig == nil || tp.TLSClientConfig.ServerName != "cdn.ageofempires.com" {
		t.Errorf("TLS ServerName not set to the original host")
	}
}

func TestNewProxyWithInternetResolutionFails(t *testing.T) {
	withCanUseInternet(t, true)
	stub := &stubResolver{dnsErr: errors.New("no IP found")}
	withStubResolver(t, stub)

	p := NewProxy("nonexistent.example.com", nil)
	if p.proxy != nil {
		t.Fatal("proxy should stay nil when upstream resolution fails")
	}
}
