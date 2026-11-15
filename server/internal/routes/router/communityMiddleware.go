package router

import (
	"context"
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// CommunityGameMiddleware resolves which game a /community request targets and
// stores it in the request context, so every handler reads it exactly like a
// /game handler does:
//
//	game := models.G(r)
//
// The title is therefore parsed and validated in one place instead of in every
// handler (§D4). The context key is models.GameContextKey and the stored value
// is a models.Game, i.e. the same pair TitleMiddleware uses, so models.G,
// models.Gg and the existing helpers keep working unchanged.
//
// Resolution rules:
//
//   - no title: the game of the listener, which TitleMiddleware already stored.
//     Nothing is re-resolved and the request is served as-is.
//   - title equal to the listener's own title: served.
//   - a title that is a supported game but not this listener's, or a title that
//     is not a supported game at all: rejected with result.code 2.
//
// The last case is where phase 6 of the plan plugs in: resolving
// initializer.Games[title] for another game this process has initialised is a
// change in this function only, with no handler touching the title at all. It is
// deliberately not enabled yet: the official service serves one title per host,
// and answering another title's data from this listener would be a behaviour
// change nobody asked for.
//
// A rejected title never reaches a handler, so handlers carry no "no game"
// branch and cannot dereference a nil game. The rejection body is a bare result
// envelope on purpose: the official service answers 404 with no body at all, so
// a parseable {"result":{"code":2,...}} is strictly more useful. An error is not
// a stub, so it does not owe the endpoint shape (§0.3 only constrains stubs).
func CommunityGameMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title := r.URL.Query().Get("title")
		if title == "" {
			next.ServeHTTP(w, r)
			return
		}
		listenerGame := models.G(r)
		if title != listenerGame.Title() {
			rejectUnknownTitle(w)
			return
		}
		ctx := context.WithValue(r.Context(), models.GameContextKey, listenerGame)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rejectUnknownTitle answers a title this listener cannot serve. The status is
// 200 with result.code 2 rather than the official 404, so the failure is
// diagnosable and a documented path never disappears (§D4).
func rejectUnknownTitle(w http.ResponseWriter) {
	i.JSON(&w, shared.Error())
}
