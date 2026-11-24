package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type dispatcherSessions struct {
	models.Sessions
	ids map[string]bool
}

func (s dispatcherSessions) GetById(id string) (models.Session, bool) {
	return nil, s.ids[id]
}

type dispatcherGame struct {
	models.Game
	title    string
	sessions models.Sessions
}

func (g *dispatcherGame) Title() string             { return g.title }
func (g *dispatcherGame) Sessions() models.Sessions { return g.sessions }

func newDispatcherFixture(t *testing.T) (*Dispatcher, map[string]*bytesRecorder) {
	t.Helper()
	games := map[string]models.Game{
		"age1": &dispatcherGame{title: "age1", sessions: dispatcherSessions{ids: map[string]bool{"s1": true}}},
		"age2": &dispatcherGame{title: "age2", sessions: dispatcherSessions{ids: map[string]bool{"s2": true}}},
		"age4": &dispatcherGame{title: "age4", sessions: dispatcherSessions{}},
	}
	handlers := make(map[string]http.Handler, len(games))
	recorders := make(map[string]*bytesRecorder, len(games))
	for id := range games {
		recorders[id] = &bytesRecorder{}
		handlers[id] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			recorders[id].body = string(body)
			recorders[id].postForm = r.PostForm
			w.Header().Set("X-Game", id)
			w.WriteHeader(http.StatusOK)
		})
	}
	resolver := &GameResolver{
		GameSet: mapset.NewThreadUnsafeSet[string]("age1", "age2", "age4"),
		DomainToGameIDs: map[string][]string{
			"ed603.playfabapi.com": {"age4"},
			"shared.example.com":   {"age1", "age2"},
		},
		UnservedHosts: map[string]bool{"api.disabled.com": true},
		Games:         games,
	}
	return NewDispatcher(handlers, resolver), recorders
}

type bytesRecorder struct {
	body     string
	postForm map[string][]string
}

func TestDispatcherPathTable(t *testing.T) {
	dispatcher, _ := newDispatcherFixture(t)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET / = %d, want 404", rr.Code)
	}
	req = httptest.NewRequest("GET", "/favicon.ico", nil)
	rr = httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /favicon.ico = %d, want 404", rr.Code)
	}
	req = httptest.NewRequest("GET", "/game/x", nil)
	req.Host = "unknown.example"
	rr = httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("multi-game without clue = %d, want 400", rr.Code)
	}
}

func TestDispatcherTitleAndHost(t *testing.T) {
	dispatcher, _ := newDispatcherFixture(t)
	cases := []struct {
		name   string
		method string
		path   string
		host   string
		body   string
		want   int
		game   string
	}{
		{"title", "GET", "/game/x?title=age2", "unknown.example", "", 200, "age2"},
		{"unknown title", "GET", "/game/x?title=age9", "unknown.example", "", 400, ""},
		{"host conflict", "GET", "/game/x?title=age2", "ed603.playfabapi.com", "", 400, ""},
		{"exclusive host", "GET", "/game/x", "ed603.playfabapi.com", "", 200, "age4"},
		{"shared host", "GET", "/game/x", "shared.example.com", "", 400, ""},
		{"session age2", "GET", "/game/x?sessionID=s2", "unknown.example", "", 200, "age2"},
		{"session conflict", "GET", "/game/x?title=age1&sessionID=s2", "unknown.example", "", 400, ""},
		{"unserved host", "GET", "/game/x?title=age1", "api.disabled.com", "", 404, ""},
		{"form title", "POST", "/game/login/platformlogin", "unknown.example", "title=age1", 200, "age1"},
		{"cdn rl path", "GET", "/aoe/rl-server-status.json", "cdn.ageofempires.com", "", 200, "age1"},
		{"cdn athens path without AoM", "GET", "/aoe/athens-server-status.json", "cdn.ageofempires.com", "", 400, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Host = tc.host
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rr := httptest.NewRecorder()
			dispatcher.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("code = %d, want %d", rr.Code, tc.want)
			}
			if tc.game != "" && rr.Header().Get("X-Game") != tc.game {
				t.Fatalf("game = %q, want %q", rr.Header().Get("X-Game"), tc.game)
			}
		})
	}
}

func TestDispatcherCDNUnknownMarkerFallsBackToTheHost(t *testing.T) {
	games := map[string]models.Game{"age2": &dispatcherGame{title: "age2"}}
	handlers := map[string]http.Handler{"age2": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Game", "age2")
		w.WriteHeader(http.StatusOK)
	})}
	resolver := &GameResolver{
		GameSet:         mapset.NewThreadUnsafeSet[string]("age2"),
		DomainToGameIDs: map[string][]string{},
		UnservedHosts:   map[string]bool{},
		Games:           games,
	}
	dispatcher := NewDispatcher(handlers, resolver)
	req := httptest.NewRequest("GET", "/aoe/desconocido.zip", nil)
	req.Host = common.CdnAgeOfEmpires
	rr := httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("X-Game") != "age2" {
		t.Fatalf("single game + unknown CDN marker = %d game=%q, want the game mux", rr.Code, rr.Header().Get("X-Game"))
	}
}

func TestDispatcherLibraryOpsPaths(t *testing.T) {
	dispatcher, _ := newDispatcherFixture(t)
	for _, path := range []string{"/test", "/test/api", "/cacert.pem", "/cacert.pem/api"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Host = "unknown.example"
		rr := httptest.NewRecorder()
		dispatcher.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s = %d, want the library path served through a game mux", path, rr.Code)
		}
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "api.disabled.com"
	rr := httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unserved host + /test = %d, want 404", rr.Code)
	}
}

func TestDispatcherAPIBodyPeekAndRestore(t *testing.T) {
	dispatcher, recorders := newDispatcherFixture(t)
	body := `{"gameTitleId":"age2","payload":"ten KB"}`
	req := httptest.NewRequest("POST", "/api/v4/Mods/Search?page=1", strings.NewReader(body))
	req.Host = "unknown.example"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("X-Game") != "age2" {
		t.Fatalf("api peek resolved game=%q code=%d", rr.Header().Get("X-Game"), rr.Code)
	}
	if got := recorders["age2"].body; got != body {
		t.Fatalf("handler body = %q, want it byte for byte", got)
	}

	req = httptest.NewRequest("POST", "/api/v4/Mods/Search", strings.NewReader(`{"gameTitleId":"age9"}`))
	req.Host = "unknown.example"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown body title = %d, want 400 (no host fallback)", rr.Code)
	}

	req = httptest.NewRequest("POST", "/api/v4/Mods/Search", strings.NewReader(`{"gameTitleId":"age1","title":"age2"}`))
	req.Host = "unknown.example"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("conflicting body titles = %d, want 400", rr.Code)
	}
}

func TestDispatcherFormLimits(t *testing.T) {
	dispatcher, _ := newDispatcherFixture(t)
	huge := "title=age1&pad=" + strings.Repeat("a", 2<<20)
	req := httptest.NewRequest("POST", "/game/login/platformlogin", strings.NewReader(huge))
	req.Host = "unknown.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	dispatcher.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("2 MB form = %d, want 413", rr.Code)
	}
}

func TestRootRoutesAnnounce(t *testing.T) {
	dispatcher, _ := newDispatcherFixture(t)
	i.AnnounceMessageData = map[string]i.AnnounceMessageDataLatest{
		"age1":   {GameTitle: "age1", Version: "test"},
		"age2":   {GameTitle: "age2", Version: "test"},
		"age4":   {GameTitle: "age4", Version: "test"},
		"athens": {GameTitle: "athens", Version: "test"},
	}
	root := RootRoutes(common.CertModeModern, nil, dispatcher, dispatcher.Resolver.Resolve)

	req := httptest.NewRequest("GET", "/"+common.AnnouncePath, nil)
	req.Host = "unknown.example"
	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("multigame announce without title = %d, want 400", rr.Code)
	}

	req = httptest.NewRequest("GET", "/"+common.AnnouncePath+"?title=age2", nil)
	req.Host = "unknown.example"
	rr = httptest.NewRecorder()
	root.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("announce with title = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"Mode":"modern"`) {
		t.Fatalf("announce did not carry the mode: %s", rr.Body.String())
	}

	req = httptest.NewRequest("GET", "/"+common.AnnouncePath+"?title=age2", nil)
	req.Host = "api.disabled.com"
	rr = httptest.NewRecorder()
	root.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("announce on an unserved host = %d, want 404", rr.Code)
	}
}
