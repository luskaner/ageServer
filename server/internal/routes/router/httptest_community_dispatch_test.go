package router

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

// TestHostDispatch_CommunityVersusGame checks the conditional-handler order of
// §D2 through the real middleware: the same listener serves /community/ without
// a session and keeps /game/ session-protected.
func TestHostDispatch_CommunityVersusGame(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	// HostMiddleware alone: TitleMiddleware would read initializer.Games, which
	// only the real startup path populates. The game is injected into the request
	// context instead, exactly as a test of the dispatch rule needs.
	handler := HostMiddleware(game.AoE2, io.Discard)

	serve := func(path, host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		req = req.WithContext(context.WithValue(req.Context(), "game", g))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	// The official host for the AoE titles; the LAN server resolves it here.
	const officialHost = "aoe-api.worldsedgelink.com"

	if recorder := serve("/community/news/getNews?title=age2", officialHost); recorder.Code != http.StatusOK {
		t.Errorf("GET /community/news/getNews: status = %d, want 200 (body=%q)",
			recorder.Code, recorder.Body.String())
	}
	// /game/news/getNews is deliberately anonymous, so a protected route is used
	// to prove the new tree did not loosen the game one.
	if recorder := serve("/game/advertisement/findAdvertisements", officialHost); recorder.Code != http.StatusUnauthorized {
		t.Errorf("GET /game/advertisement/findAdvertisements: status = %d, want 401", recorder.Code)
	}
	if recorder := serve("/community/news/getNews", "andromeda-live-release21-api.worldsedgelink.com"); recorder.Code != http.StatusOK {
		t.Errorf("AoM host /community: status = %d, want 200", recorder.Code)
	}
}
