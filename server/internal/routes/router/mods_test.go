package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// fakeMods echoes the path it received, so a test can tell whether the request
// reached the mods API or something else.
type fakeMods struct{}

func (fakeMods) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})
}

func (fakeMods) Shutdown(context.Context) error { return nil }

// withMods installs the fake library for the duration of one test.
func withMods(t *testing.T) {
	t.Helper()
	oldAPI := i.ModsAPI
	oldEnabled := i.ModsEnabled
	i.ModsAPI = fakeMods{}
	i.ModsEnabled = true
	t.Cleanup(func() {
		i.ModsAPI = oldAPI
		i.ModsEnabled = oldEnabled
	})
}

// apiHandler builds the api.ageofempires.com handler. The proxy is nil without
// internet, so an unmatched path answers 404 from the mux instead of dialling
// upstream.
func apiHandler() http.Handler {
	a := NewApiAgeOfEmpires()
	return a.InitializeRoutes(game.AoE1, http.NotFoundHandler())
}

// TestApiAgeOfEmpiresHandsTheModsTreeToTheLibrary covers both casings the
// clients disagree on, the legacy v1 tree and the polls.
func TestApiAgeOfEmpiresHandsTheModsTreeToTheLibrary(t *testing.T) {
	withMods(t)
	handler := apiHandler()

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v4/Mods/Games"},
		{http.MethodPost, "/api/v4/mods/Find/"},
		{http.MethodGet, "/api/v4/Mods/Detail/583126/"},
		{http.MethodPost, "/api/v1/Mods/Detail/"},
		{http.MethodGet, "/Poll/list"},
	}
	for _, tc := range cases {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		if got := recorder.Body.String(); got != tc.path {
			t.Fatalf("%s %s reached %q, want the mods API", tc.method, tc.path, got)
		}
	}
}

// TestAoe4HostHandsTheModsTreeToTheLibrary covers the AoE IV host, which
// reuses the handler of the generic one.
func TestAoe4HostHandsTheModsTreeToTheLibrary(t *testing.T) {
	withMods(t)
	handler := NewAoe4ApiAgeOfEmpires().InitializeRoutes(game.AoE4, http.NotFoundHandler())

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Download/583126/file", nil))
	if got := recorder.Body.String(); got != "/api/v4/Mods/Download/583126/file" {
		t.Fatalf("request reached %q, want the mods API", got)
	}
}

// TestApiAgeOfEmpiresHandsMatchContentToTheLibrary checks the savegame tree is
// served by the same instance, before the proxy fallback. The test request
// carries a decodable ticket, so it is an emulated client (a request without
// credentials would be an official one and go upstream as soon as there is
// internet).
func TestApiAgeOfEmpiresHandsMatchContentToTheLibrary(t *testing.T) {
	withMods(t)
	handler := apiHandler()

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/MatchContent/"},
		{http.MethodGet, "/api/MatchContent/234876462"},
		{http.MethodGet, "/API/MatchContent/234876462/file/deadbeef"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)
		request.Header.Set("Authorization", "Steam1.0 "+modsSteamToken(76561198000000456))
		handler.ServeHTTP(recorder, request)
		if got := recorder.Body.String(); got != tc.path {
			t.Fatalf("%s %s reached %q, want the library", tc.method, tc.path, got)
		}
	}
}

// TestGameHostHandsMatchContentToTheLibrary covers the titles that reach the
// endpoints through the game host, and that the mods tree is not mounted there.
func TestGameHostHandsMatchContentToTheLibrary(t *testing.T) {
	withMods(t)
	g := &Game{}
	handler := g.InitializeRoutes(game.AoE2, nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/MatchContent/42", nil))
	if got := recorder.Body.String(); got != "/api/MatchContent/42" {
		t.Fatalf("MatchContent reached %q, want the library", got)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil))
	if recorder.Code == http.StatusOK && recorder.Body.String() == "/api/v4/Mods/Games" {
		t.Fatal("the mods tree must not be mounted on the game host")
	}
}

// TestMatchContentStaysLocalWhenModsAreDisabled pins the decoupling: the
// library is built for [CustomFiles] alone, so MatchContent keeps working
// while the mods tree follows the proxy path again.
func TestMatchContentStaysLocalWhenModsAreDisabled(t *testing.T) {
	withMods(t)
	i.ModsEnabled = false
	handler := apiHandler()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/MatchContent/", nil))
	if got := recorder.Body.String(); got != "/api/MatchContent/" {
		t.Fatalf("MatchContent reached %q, want the library", got)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("mods path code = %d, want the mux 404 with the feature off", recorder.Code)
	}
}

// TestApiAgeOfEmpiresKeepsTextModerationLocal checks the mock that was already
// there is still the one answering, not the mods API.
func TestApiAgeOfEmpiresKeepsTextModerationLocal(t *testing.T) {
	withMods(t)
	handler := apiHandler()

	body := strings.NewReader(`{"textType":"SanitisationUsername"}`)
	request := httptest.NewRequest(http.MethodPost, "/textmoderation", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Body.String(); !strings.Contains(got, "Allow") {
		t.Fatalf("body = %q, want the local moderation answer", got)
	}
}

// TestApiAgeOfEmpiresWithoutModsForwardsNothing pins the rollback: with the
// feature off the paths are not registered here, so the proxy keeps them.
func TestApiAgeOfEmpiresWithoutModsForwardsNothing(t *testing.T) {
	old := i.ModsAPI
	i.ModsAPI = nil
	t.Cleanup(func() { i.ModsAPI = old })

	handler := apiHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v4/mods/Games", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 from the mux", recorder.Code)
	}
}

// TestClientDispatchKeepsTheProxyForSelectedClients covers both branches of
// the client condition: the selected client goes upstream, the rest stays local.
func TestClientDispatchKeepsTheProxyForSelectedClients(t *testing.T) {
	old := proxyModsRequest
	t.Cleanup(func() { proxyModsRequest = old })

	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream"))
	})
	local := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("local"))
	})

	proxyModsRequest = func(string, *http.Request) bool { return true }
	recorder := httptest.NewRecorder()
	clientDispatch(game.AoE2, local, upstream).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil))
	if got := recorder.Body.String(); got != "upstream" {
		t.Fatalf("selected client reached %q, want the upstream proxy", got)
	}

	proxyModsRequest = func(string, *http.Request) bool { return false }
	recorder = httptest.NewRecorder()
	clientDispatch(game.AoE2, local, upstream).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil))
	if got := recorder.Body.String(); got != "local" {
		t.Fatalf("unselected client reached %q, want the mods API", got)
	}
}

// TestClientDispatchWithoutUpstreamAnswersLocally pins the offline fallback: a
// selected client still gets a real answer when there is nothing to forward to.
func TestClientDispatchWithoutUpstreamAnswersLocally(t *testing.T) {
	old := proxyModsRequest
	t.Cleanup(func() { proxyModsRequest = old })
	proxyModsRequest = func(string, *http.Request) bool { return true }

	local := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("local"))
	})
	recorder := httptest.NewRecorder()
	clientDispatch(game.AoE2, local, nil).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil))
	if got := recorder.Body.String(); got != "local" {
		t.Fatalf("body = %q, want the local answer without internet", got)
	}
}

// TestMatchContentFollowsTheClientCondition pins the user decision that the
// whole api host follows the official-client rule, not only the mods tree:
// savegame requests of an official client go upstream with internet, and those
// of an emulated client keep the local library even then. The game host passes
// no upstream, so it stays local for everyone.
func TestMatchContentFollowsTheClientCondition(t *testing.T) {
	withMods(t)
	oldInternet := i.CanUseInternet
	t.Cleanup(func() { i.CanUseInternet = oldInternet })
	i.CanUseInternet = true

	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream"))
	})
	apiHost := withAgeAPIServer(http.NotFoundHandler(), game.AoE2, upstream, true)
	gameHost := withAgeAPIServer(http.NotFoundHandler(), game.AoE2, nil, false)

	// No credentials: indistinguishable from an official client, goes upstream.
	recorder := httptest.NewRecorder()
	apiHost.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/MatchContent/", nil))
	if got := recorder.Body.String(); got != "upstream" {
		t.Fatalf("official client MatchContent reached %q, want the upstream proxy", got)
	}

	// A decodable (gbe_fork) ticket means an emulated client: local answer.
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/MatchContent/", nil)
	request.Header.Set("Authorization", "Steam1.0 "+modsSteamToken(76561198000000456))
	apiHost.ServeHTTP(recorder, request)
	if got := recorder.Body.String(); got != "/api/MatchContent/" {
		t.Fatalf("emulated client MatchContent reached %q, want the library", got)
	}

	// The game host was never proxied: same ticket shape, still local.
	recorder = httptest.NewRecorder()
	gameHost.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/MatchContent/", nil))
	if got := recorder.Body.String(); got != "/api/MatchContent/" {
		t.Fatalf("game host MatchContent reached %q, want the library", got)
	}
}

// TestIsPublishPath covers the casing and the trailing slash of the only
// endpoint whose body needs a raised read deadline.
func TestIsPublishPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/v4/mods/Publish", true},
		{"/api/v4/Mods/publish/", true},
		{"/API/V4/MODS/PUBLISH", true},
		{"/api/v4/mods/Find/", false},
		{"/api/v4/mods/published", false},
	}
	for _, tc := range cases {
		if got := isPublishPath(tc.path); got != tc.want {
			t.Fatalf("isPublishPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestApiHostsRewriteTheLibraryOpsRoutes covers the public names of the
// library's ops routes: the request is rewritten onto the path the library
// registers, and it is the library that answers. The fake library echoes the
// path it received, so the body proves which path arrived.
func TestApiHostsRewriteTheLibraryOpsRoutes(t *testing.T) {
	withMods(t)
	handler := apiHandler()

	for _, tc := range []struct{ public, library string }{
		{"/test/api", "/test"},
		{"/test/api/", "/test"},
		{"/TEST/API", "/test"},
		{"/cacert.pem/api", "/cacert.pem"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.public, nil))
		if got := recorder.Body.String(); got != tc.library {
			t.Fatalf("%s reached %q, want the library path %q", tc.public, got, tc.library)
		}
	}
}

// TestApiHostsLeaveTheBareOpsPathsToTheHost pins that only the public names are
// rewritten: the bare /test and /cacert.pem are not a second way in, they keep
// whatever the host handler did with them.
func TestApiHostsLeaveTheBareOpsPathsToTheHost(t *testing.T) {
	withMods(t)
	handler := apiHandler()

	for _, path := range []string{"/test", "/cacert.pem"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s code = %d, want the mux 404", path, recorder.Code)
		}
	}
}

// TestApiHostsWithoutTheLibraryDoNotRewrite pins the rollback: with no library
// there is nothing to rewrite to, so the public paths are not registered.
func TestApiHostsWithoutTheLibraryDoNotRewrite(t *testing.T) {
	old := i.ModsAPI
	i.ModsAPI = nil
	t.Cleanup(func() { i.ModsAPI = old })

	handler := apiHandler()
	for _, path := range []string{"/test/api", "/cacert.pem/api"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s code = %d, want the mux 404 without the library", path, recorder.Code)
		}
	}
}
