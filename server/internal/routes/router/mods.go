package router

import (
	"net/http"
	"path"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

// modsRoots are the trees the embedded ageAPIServer library owns on the Age of
// Empires hosts. They are matched in one place, not registered one by one on
// the mux: the library normalises the casing and the trailing slash itself, and
// a ServeMux pattern cannot match both /api/v4/Mods and /api/v4/mods.
var modsRoots = []string{"/api/v4", "/api/v1", "/poll"}

// matchContentRoot is the savegame/replay tree the same library serves. It is
// separate from modsRoots because it is also mounted on the game host and never
// follows the upstream proxy decision.
const matchContentRoot = "/api/MatchContent"

// withAgeAPIServer mounts the embedded library once, in front of the handler of
// a host. Everything the library owns is matched here (case-insensitively and
// with the trailing slash optional) and the rest is left to the host handler,
// which keeps the proxy fallback and the routes that were already local.
//
// Both trees the library owns follow the same client condition, because the
// reason behind it does not change with the tree: an official client keeps the
// official service (mods and savegame exchange alike), an emulated or offline
// client gets the local answer. It is no accident that the game host passes no
// upstream: that host was never proxied, so every request it receives stays
// local.
//
// serveMods is false on the game host: that host only carries MatchContent, so
// the mods tree keeps whatever it did before this feature existed.
func withAgeAPIServer(host http.Handler, gameID string, upstream http.Handler, serveMods bool) http.Handler {
	server := i.ModsAPI
	if server == nil {
		return host
	}
	local := modsHandler(server.Handler())
	byClient := clientDispatch(gameID, local, upstream)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case isMatchContentPath(r.URL.Path):
			byClient.ServeHTTP(w, r)
		case serveMods && i.ModsEnabled && isModsPath(r.URL.Path):
			byClient.ServeHTTP(w, r)
		default:
			host.ServeHTTP(w, r)
		}
	})
}

// isModsPath reports whether the path belongs to the mods tree, ignoring the
// casing and the trailing slash the clients disagree on.
func isModsPath(urlPath string) bool {
	cleaned := ""
	for _, root := range modsRoots {
		if !hasPrefixFold(urlPath, root) {
			continue
		}
		if cleaned == "" {
			cleaned = strings.ToLower(path.Clean(urlPath))
		}
		if hasRootOrSubtree(cleaned, root) {
			return true
		}
	}
	return false
}

// isMatchContentPath reports whether the path addresses the savegame endpoints.
func isMatchContentPath(urlPath string) bool {
	if !hasPrefixFold(urlPath, matchContentRoot) {
		return false
	}
	return hasRootOrSubtree(strings.ToLower(path.Clean(urlPath)), strings.ToLower(matchContentRoot))
}

// hasRootOrSubtree reports whether cleaned is root itself or a path below it.
func hasRootOrSubtree(cleaned, root string) bool {
	return cleaned == root || strings.HasPrefix(cleaned, root+"/")
}

// hasPrefixFold is strings.HasPrefix ignoring ASCII case, without allocating a
// lowercase copy of every request path.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

// proxyModsRequest selects the official service when the server has internet
// and the client is official. A client is not official exactly when its
// `Steam1.0` ticket can be decoded: gbe_fork emits plaintext auth tickets,
// while a real Steam ticket is opaque. Every other request (no token, another
// scheme, an opaque ticket) is treated as official and goes upstream. The rule
// governs every tree the library serves, mods and MatchContent alike.
//
// It is a variable so a test can drive both branches and so the condition can
// be replaced without touching the dispatch.
var proxyModsRequest = func(_ string, r *http.Request) bool {
	if !i.CanUseInternet {
		return false
	}
	token, ok := steamToken(r)
	if !ok {
		return true
	}
	_, decodable := playfab.SteamIDFromToken(token)
	return !decodable
}

// steamToken returns the token of an `Authorization: Steam1.0 <token>` header.
// The scheme has a dot, so r.BasicAuth() cannot parse it.
func steamToken(r *http.Request) (string, bool) {
	const scheme = "Steam1.0 "
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), scheme)
	token = strings.TrimSpace(token)
	return token, found && token != ""
}

// clientDispatch decides per request between the local library and the
// upstream proxy, for every tree the library serves. Without internet there is
// nothing to forward to, so the local answer wins: a selected client gets a
// real response instead of a 404.
func clientDispatch(gameID string, local http.Handler, upstream http.Handler) http.Handler {
	if upstream == nil {
		return local
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if proxyModsRequest(gameID, r) {
			upstream.ServeHTTP(w, r)
			return
		}
		local.ServeHTTP(w, r)
	})
}

// modsHandler wraps the library so a streaming upload gets a read deadline
// sized for its body. The server-wide ReadTimeout only covers the small form
// posts the game normally makes, and a mod archive or savegame would be cut off
// before it finished arriving.
func modsHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			switch {
			case i.CustomFilesConfig.Enabled && isMatchContentPath(r.URL.Path):
				i.ExtendReadDeadline(w, i.CustomFilesConfig.MaxFileSize)
			case i.ModsEnabled && isPublishPath(r.URL.Path):
				i.ExtendReadDeadline(w, i.ModsConfig.MaxUploadSize)
			}
		}
		handler.ServeHTTP(w, r)
	})
}

// isPublishPath reports whether the request addresses the Publish endpoint,
// ignoring the casing and the trailing slash the clients disagree on.
func isPublishPath(urlPath string) bool {
	trimmed := strings.TrimSuffix(urlPath, "/")
	base := strings.ToLower(path.Base(trimmed))
	return base == "publish" || base == "create"
}
