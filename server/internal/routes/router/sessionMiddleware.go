package router

import (
	"context"
	"net/http"
	"strings"

	"github.com/luskaner/ageServer/server/internal/models"
)

var sessAnonymousPaths = map[string]bool{
	"/game/msstore/getStoreTokens":      true,
	"/game/login/platformlogin":         true,
	"/wss/":                             true,
	"/game/news/getNews":                true,
	"/game/Challenge/getChallenges":     true,
	"/game/item/getItemBundleItemsJson": true,
}

// sessAnonymousPrefixes are path prefixes served without a session. The game
// does not send sessionID on custom-file (savegame/replay) requests, exactly
// like it does not on cloudfiles.
var sessAnonymousPrefixes = []string{"/cloudfiles/", "/api/MatchContent/"}

func isAnonymousPath(path string) bool {
	if sessAnonymousPaths[path] {
		return true
	}
	for _, prefix := range sessAnonymousPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAnonymousPath(r.URL.Path) {
			sessionID := r.FormValue("sessionID")
			if sessionID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			sessions := models.G(r).Sessions()
			sess, ok := sessions.GetById(sessionID)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			sessions.ResetExpiry(sess.Id())
			ctx := context.WithValue(r.Context(), "session", sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
