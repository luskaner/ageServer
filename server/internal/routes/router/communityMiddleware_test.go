package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/models"
)

// TestCommunityGameMiddleware_SetsGameForHandler locks the contract of the
// middleware that replaced the per-handler title parsing: a handler under
// /community reads the game through models.G(r), exactly like a /game handler,
// and never has to look at the title parameter.
func TestCommunityGameMiddleware_SetsGameForHandler(t *testing.T) {
	g := newTestGame(t, game.AoE2)

	var seen models.Game
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = models.G(r)
		w.WriteHeader(http.StatusOK)
	})
	handler := CommunityGameMiddleware(next)

	for _, path := range []string{
		"/community/news/getNews",
		"/community/news/getNews?title=age2",
	} {
		seen = nil
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(context.WithValue(req.Context(), models.GameContextKey, g))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, recorder.Code)
		}
		if seen == nil {
			t.Fatalf("%s: handler saw no game in the context", path)
		}
		if seen.Title() != game.AoE2 {
			t.Fatalf("%s: handler saw game %q, want %q", path, seen.Title(), game.AoE2)
		}
	}
}

// TestCommunityGameMiddleware_RejectsForeignTitle verifies that a title the
// listener does not serve never reaches a handler: the middleware answers with
// result.code 2 and HTTP 200 instead of letting a handler dereference a game it
// does not own (§D4).
func TestCommunityGameMiddleware_RejectsForeignTitle(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	called := false
	handler := CommunityGameMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	// A supported game this listener does not serve, and a value that is not a
	// game at all, are rejected alike. An empty title is not in this list on
	// purpose: it means "the listener's game" and passes through.
	for _, title := range []string{"age4", "athens", "age9", "notagame"} {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/community/news/getNews?title="+title, nil)
		req = req.WithContext(context.WithValue(req.Context(), models.GameContextKey, g))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("title=%q: status = %d, want 200", title, recorder.Code)
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Errorf("title=%q: body is not JSON: %v", title, err)
			continue
		}
		result, ok := body["result"].(map[string]any)
		if !ok {
			t.Errorf("title=%q: result is not an object: %v", title, body)
			continue
		}
		if code, _ := result["code"].(float64); int(code) != 2 {
			t.Errorf("title=%q: result.code = %v, want 2", title, result["code"])
		}
		if called {
			t.Errorf("title=%q: handler ran for a rejected title", title)
		}
	}
}
