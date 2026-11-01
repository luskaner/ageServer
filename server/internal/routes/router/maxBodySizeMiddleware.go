package router

import (
	"net/http"
	"path"
	"strings"
)

// raisedBodySizePrefixes are paths that get their own (larger) body limit from
// the handler. The global 1 MiB cap is skipped for them so the inner
// MaxBytesReader can raise - not lower - the effective limit.
var raisedBodySizePrefixes = []string{"/api/MatchContent/"}

// raisedBodySizePath matches against the cleaned path, because this runs before
// ServeMux normalises it: a raw "/api/MatchContent/../game/..." would otherwise
// skip the cap on a path the cap is meant to cover.
func raisedBodySizePath(urlPath string) bool {
	// Clean drops a trailing slash, so the prefix is compared without it and
	// reattached: the upload route is the bare "/api/MatchContent/", and
	// matching only the slash-terminated form would leave the global cap in
	// front of it.
	cleaned := path.Clean(urlPath)
	for _, prefix := range raisedBodySizePrefixes {
		trimmed := strings.TrimSuffix(prefix, "/")
		if cleaned == trimmed || strings.HasPrefix(cleaned, trimmed+"/") {
			return true
		}
	}
	return false
}

func MaxBodySizeMiddleware(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !raisedBodySizePath(r.URL.Path) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}
