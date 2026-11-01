package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/uuid"
)

func TestTlsConfigWithNilRootCAs(t *testing.T) {
	cfg := TlsConfig("example.com", false, nil)
	if cfg == nil {
		t.Fatal("TlsConfig returned nil")
	}
	if cfg.ServerName != "example.com" {
		t.Errorf("ServerName = %q, want %q", cfg.ServerName, "example.com")
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}
	// RootCAs may be nil if certStore.CertPool() returns nil (no certs installed)
	// The key is that it uses certStore.CertPool() when nil is passed
}

func TestTlsConfigWithCustomRootCAs(t *testing.T) {
	pool := x509.NewCertPool()
	cfg := TlsConfig("server.local", true, pool)
	if cfg.ServerName != "server.local" {
		t.Errorf("ServerName = %q, want %q", cfg.ServerName, "server.local")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
	if cfg.RootCAs != pool {
		t.Error("RootCAs should be the custom pool")
	}
}

func TestTlsConfigEmptyServerName(t *testing.T) {
	cfg := TlsConfig("", false, nil)
	if cfg.ServerName != "" {
		t.Errorf("ServerName = %q, want empty", cfg.ServerName)
	}
}

func TestLatencyMeasurementCount(t *testing.T) {
	if LatencyMeasurementCount != 3 {
		t.Errorf("LatencyMeasurementCount = %d, want 3", LatencyMeasurementCount)
	}
}

func TestAnnounceMessageDataSupportedLatest(t *testing.T) {
	// Type alias should compile and be usable
	var _ AnnounceMessageDataSupportedLatest
}

// Integration: LanServerIP success via httptest TLSServer
func TestLanServerIPIntegrationSuccess(t *testing.T) {
	serverId := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	gameTitle := "age2"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(common.VersionHeader, "2")
		w.Header().Set(common.IdHeader, serverId.String())
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle, Version: "1.11.0"})
	})
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()

	// Extract host:port from ts.URL (https://127.0.0.1:xxxxx)
	u, _ := url.Parse(ts.URL)
	hostPort := u.Host // 127.0.0.1:xxxxx
	host, port, _ := net.SplitHostPort(hostPort)
	ip := net.ParseIP(host)
	if ip == nil {
		t.Fatalf("invalid ip from test server: %s", host)
	}

	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	origPort := serverPort
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient; serverPort = origPort }()

	serverPort = port
	buildURLFn = func(_ net.IP) url.URL {
		return url.URL{Scheme: "https", Host: hostPort, Path: "test"}
	}
	newHTTPClientFn = func(serverName string, insecureSkipVerify bool, rootCAs *x509.CertPool) *http.Client {
		// Use test server's client which trusts its cert; ignore args
		return ts.Client()
	}

	ok, gotId, _, data := LanServerIP(uuid.Nil(), gameTitle, ip, "test.local", true, nil, true)
	if !ok {
		t.Fatal("LanServerIP should succeed")
	}
	if gotId != serverId {
		t.Errorf("serverId = %v, want %v", gotId, serverId)
	}
	if data == nil || data.GameTitle != gameTitle {
		t.Errorf("data = %v, want GameTitle %q", data, gameTitle)
	}
}

func TestLanServerIPIntegrationWithLatency(t *testing.T) {
	serverId := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	gameTitle := "age2"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(common.VersionHeader, "2")
		w.Header().Set(common.IdHeader, serverId.String())
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle, Version: "1.0"})
	})
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	hostPort := u.Host
	host, port, _ := net.SplitHostPort(hostPort)
	ip := net.ParseIP(host)

	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	origPort := serverPort
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient; serverPort = origPort }()
	serverPort = port
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: hostPort, Path: "test"} }
	newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client { return ts.Client() }

	ok, _, latency, _ := LanServerIP(uuid.Nil(), gameTitle, ip, "test.local", true, nil, false)
	if !ok {
		t.Fatal("LanServerIP with latency should succeed")
	}
	if latency == 0 {
		t.Error("latency should be non-zero when ignoreLatency=false")
	}
}

func TestLanServerIPIntegrationFailures(t *testing.T) {
	serverId := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	gameTitle := "age2"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		game    string
		id      uuid.UUID
	}{
		{"status not 200", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }, gameTitle, uuid.Nil()},
		{"missing version header", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.IdHeader, serverId.String())
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle})
		}, gameTitle, uuid.Nil()},
		{"missing id header", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "2")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle})
		}, gameTitle, uuid.Nil()},
		{"version too high", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "99")
			w.Header().Set(common.IdHeader, serverId.String())
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle})
		}, gameTitle, uuid.Nil()},
		{"id mismatch", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "2")
			w.Header().Set(common.IdHeader, serverId.String())
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle})
		}, gameTitle, uuid.MustParse("00000000-0000-0000-0000-000000000001")},
		{"gameTitle mismatch", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "2")
			w.Header().Set(common.IdHeader, serverId.String())
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: "wrong"})
		}, gameTitle, uuid.Nil()},
		{"invalid json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "2")
			w.Header().Set(common.IdHeader, serverId.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not json"))
		}, gameTitle, uuid.Nil()},
		{"invalid id header", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(common.VersionHeader, "2")
			w.Header().Set(common.IdHeader, "not-a-uuid")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle})
		}, gameTitle, uuid.Nil()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewTLSServer(tc.handler)
			defer ts.Close()
			u, _ := url.Parse(ts.URL)
			hostPort := u.Host
			host, port, _ := net.SplitHostPort(hostPort)
			ip := net.ParseIP(host)
			origBuildURL := buildURLFn
			origClient := newHTTPClientFn
			origPort := serverPort
			defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient; serverPort = origPort }()
			serverPort = port
			buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: hostPort, Path: "test"} }
			newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client { return ts.Client() }
			ok, _, _, _ := LanServerIP(tc.id, tc.game, ip, "test.local", true, nil, true)
			if ok {
				t.Error("expected LanServerIP to fail")
			}
		})
	}
}

func TestLanServerHostIntegration(t *testing.T) {
	serverId := uuid.MustParse("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	gameTitle := "age2"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(common.VersionHeader, "2")
		w.Header().Set(common.IdHeader, serverId.String())
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AnnounceMessageDataSupportedLatest{GameTitle: gameTitle, Version: "1.0"})
	})
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	hostPort := u.Host
	_, port, _ := net.SplitHostPort(hostPort)

	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	origPort := serverPort
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient; serverPort = origPort }()
	serverPort = port
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: hostPort, Path: "test"} }
	newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client { return ts.Client() }

	// Mock HostOrIpToIps to return 127.0.0.1 for our fake host
	origResolver := common.SetResolver(&mockResolverForServer{ips: []string{"127.0.0.1"}})
	defer origResolver()

	if !LanServerHost(uuid.Nil(), gameTitle, "fake.host", true, nil) {
		t.Error("LanServerHost should succeed with mocked resolver and test server")
	}
	// Empty host should fail
	if LanServerHost(uuid.Nil(), gameTitle, "empty.host", true, nil) {
		t.Error("LanServerHost should fail for empty host")
	}
}

type mockResolverForServer struct {
	ips []string
}

func (m *mockResolverForServer) HostToIPs(host string) []net.IP {
	if host == "empty.host" {
		return nil
	}
	var out []net.IP
	for _, s := range m.ips {
		if ip := net.ParseIP(s); ip != nil {
			out = append(out, ip)
		}
	}
	return out
}
func (m *mockResolverForServer) IPToHosts(ip string) []string               { return nil }
func (m *mockResolverForServer) DirectHostToIP(host string) (string, error) { return "", nil }
func (m *mockResolverForServer) DialTCP(network, address string, timeout time.Duration) (net.Conn, error) {
	return nil, nil
}
func (m *mockResolverForServer) NetInterfaces() ([]net.Interface, error) { return nil, nil }
func (m *mockResolverForServer) RunningNetworkInterfaces() (map[*net.Interface][]*net.IPNet, error) {
	return nil, nil
}

func TestCheckConnectionFromServerIntegration(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	hostPort := u.Host
	host, port, _ := net.SplitHostPort(hostPort)

	origPort := serverPort
	origDial := tlsDialFn
	defer func() { serverPort = origPort; tlsDialFn = origDial }()
	serverPort = port
	origResolver := common.SetResolver(&mockResolverForServer{ips: []string{host}})
	defer origResolver()
	if err := CheckConnectionFromServer("fake.host", true, nil); err != nil {
		t.Errorf("CheckConnectionFromServer should succeed: %v", err)
	}
	conn, err := tlsDialFn("tcp4", net.JoinHostPort(host, port), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("tlsDialFn: %v", err)
	}
	_ = conn.Close()
}

func TestConnectToServerEmptyIps(t *testing.T) {
	origResolver := common.SetResolver(&mockResolverForServer{ips: nil})
	defer origResolver()
	origDial := tlsDialFn
	origPort := serverPort
	defer func() { tlsDialFn = origDial; serverPort = origPort }()
	var capturedAddr string
	tlsDialFn = func(network, address string, config *tls.Config) (*tls.Conn, error) {
		capturedAddr = address
		return nil, nil
	}
	serverPort = "8443"
	_, _ = connectToServer("empty.host", true, nil)
	if !strings.Contains(capturedAddr, "empty.host:8443") {
		t.Errorf("capturedAddr = %q, want contains empty.host:8443", capturedAddr)
	}
}

func TestLanServerHostFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	hostPort := u.Host
	_, port, _ := net.SplitHostPort(hostPort)
	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	origPort := serverPort
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient; serverPort = origPort }()
	serverPort = port
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: hostPort, Path: "test"} }
	newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client { return ts.Client() }
	origResolver := common.SetResolver(&mockResolverForServer{ips: []string{"127.0.0.1"}})
	defer origResolver()
	if LanServerHost(uuid.Nil(), "age2", "fake.host", true, nil) {
		t.Error("LanServerHost should fail when server returns 500")
	}
}

type errRoundTripper struct{}

func (errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("mock transport error")
}

func TestLanServerIPHeadDoError(t *testing.T) {
	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient }()
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: "127.0.0.1:443", Path: "test"} }
	newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client {
		return &http.Client{Transport: errRoundTripper{}, Timeout: 1 * time.Second}
	}
	ok, _, _, _ := LanServerIP(uuid.Nil(), "age2", net.ParseIP("127.0.0.1"), "test.local", true, nil, false)
	if ok {
		t.Error("LanServerIP should fail when HEAD Do returns error")
	}
}

func TestLanServerIPGetDoError(t *testing.T) {
	origBuildURL := buildURLFn
	origClient := newHTTPClientFn
	defer func() { buildURLFn = origBuildURL; newHTTPClientFn = origClient }()
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: "https", Host: "127.0.0.1:443", Path: "test"} }
	newHTTPClientFn = func(string, bool, *x509.CertPool) *http.Client {
		return &http.Client{Transport: errRoundTripper{}, Timeout: 1 * time.Second}
	}
	ok, _, _, _ := LanServerIP(uuid.Nil(), "age2", net.ParseIP("127.0.0.1"), "test.local", true, nil, true)
	if ok {
		t.Error("LanServerIP should fail when GET Do returns error")
	}
}

func TestLanServerIPNewRequestError(t *testing.T) {
	origBuildURL := buildURLFn
	defer func() { buildURLFn = origBuildURL }()
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: ":", Host: ":", Path: ":"} }
	ok, _, _, _ := LanServerIP(uuid.Nil(), "age2", net.ParseIP("127.0.0.1"), "test.local", true, nil, true)
	if ok {
		t.Error("LanServerIP should fail when NewRequest returns error")
	}
}

func TestLanServerIPHeadNewRequestError(t *testing.T) {
	origBuildURL := buildURLFn
	defer func() { buildURLFn = origBuildURL }()
	buildURLFn = func(_ net.IP) url.URL { return url.URL{Scheme: ":", Host: ":", Path: ":"} }
	ok, _, _, _ := LanServerIP(uuid.Nil(), "age2", net.ParseIP("127.0.0.1"), "test.local", true, nil, false)
	if ok {
		t.Error("LanServerIP should fail when HEAD NewRequest returns error")
	}
}
