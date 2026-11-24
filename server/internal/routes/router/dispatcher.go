package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	commonGame "github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/playfab"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

const (
	// dispatcherFormLimit is the same cap MaxBodySizeMiddleware applies inside
	// a game mux. The dispatcher runs before it, so it has to enforce its own.
	dispatcherFormLimit = 1 << 20
	// dispatcherPeekLimit bounds the JSON body that is inspected for a title.
	dispatcherPeekLimit = 64 << 10
)

var gamePathPrefixes = []string{"/game/", "/community/", "/cloudfiles/", "/wss/", "/api/", "/poll", "/aoe/"}

var playfabPathRoots = []string{
	"/Client", "/Event", "/Inventory", "/MultiplayerServer", "/Party", "/Catalog", "/CloudScript",
}

// isGamePath reports whether the path belongs to a game tree. The root routes
// (/test/root, /cacert.pem/root, /sigkey.der, /shutdown) do not: they are
// answered by the listener itself.
func isGamePath(urlPath string) bool {
	for _, prefix := range gamePathPrefixes {
		trimmed := strings.TrimSuffix(prefix, "/")
		if urlPath == trimmed || strings.HasPrefix(urlPath, prefix) {
			return true
		}
	}
	for _, root := range playfabPathRoots {
		if urlPath == root || strings.HasPrefix(urlPath, root+"/") {
			return true
		}
	}
	return strings.HasPrefix(urlPath, playfab.StaticSuffix)
}

func isAPIPath(urlPath string) bool {
	return urlPath == "/api" || strings.HasPrefix(urlPath, "/api/") || urlPath == "/poll"
}

func isUploadPath(urlPath string) bool {
	return strings.HasPrefix(urlPath, "/cloudfiles/")
}

func isWssPath(urlPath string) bool {
	return urlPath == "/wss" || strings.HasPrefix(urlPath, "/wss/")
}

// libraryOpsPaths are the public diagnostics of the embedded Age API library.
// They do not depend on a game: the library is one instance shared by every
// enabled title, and the game's HostMiddleware decides which hosts serve it
// (the API ones) and which answer 404. Routing them through any enabled game
// keeps the paths alive; with no game enabled there is nothing to route to,
// which is the same 404 HEAD gave with the library absent.
var libraryOpsPaths = map[string]bool{
	"/test": true, "/test/api": true,
	"/cacert.pem": true, "/cacert.pem/api": true,
}

func needsForm(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	}
	return false
}

// isAPIPeekPath is the only branch that inspects an API body: a JSON mutation
// that does not carry the title in its query, on a tree where gameTitleId is a
// stable marker. MatchContent is binary and large, so it is never peeked.
func isAPIPeekPath(r *http.Request) bool {
	return isAPIPath(r.URL.Path) &&
		needsForm(r) &&
		r.URL.Query().Get("title") == "" &&
		strings.Contains(r.Header.Get("Content-Type"), "application/json") &&
		r.URL.Path != "/api/MatchContent"
}

// requestValue reads a value from the query or from a form the dispatcher has
// already parsed. It deliberately never calls FormValue: on an API path that
// would read (and consume) the body the handler has to see intact.
func requestValue(r *http.Request, key string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	if r.PostForm != nil {
		return r.PostForm.Get(key)
	}
	return ""
}

// titleFromBody returns the title a JSON body declares, without filtering it by
// the enabled games: the resolver validates it and answers 400 for an unknown or
// disabled one instead of silently falling back to the host.
func titleFromBody(body []byte) (gameID string, conflict bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	values := make(map[string]bool, 2)
	for _, key := range []string{"gameTitleId", "title"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok && str != "" {
			values[str] = true
		}
	}
	if len(values) > 1 {
		return "", true
	}
	for value := range values {
		return value, false
	}
	return "", false
}

// sessionOwner returns the enabled game whose sessions hold the id, and whether
// more than one does (an id shared by two games says nothing about the request).
func (gr *GameResolver) sessionOwner(sessionID string) (owner string, ambiguous bool) {
	if sessionID == "" {
		return "", false
	}
	for _, candidate := range gr.enabledGames() {
		game := gr.Games[candidate]
		if game == nil || game.Sessions() == nil {
			continue
		}
		if _, ok := game.Sessions().GetById(sessionID); ok {
			if owner != "" {
				return "", true
			}
			owner = candidate
		}
	}
	return owner, false
}

// GameResolver is the single resolution both the listener's root routes and the
// dispatcher use. It is a value with state, not a global, so a test can inject
// its own maps and games.
type GameResolver struct {
	GameSet         mapset.Set[string]
	DomainToGameIDs map[string][]string
	UnservedHosts   map[string]bool
	Games           map[string]models.Game
}

func NewGameResolver(gameSet mapset.Set[string], apiEnabled bool, games map[string]models.Game) *GameResolver {
	served, unserved := common.BuildDomainToGameIDs(apiEnabled)
	return &GameResolver{
		GameSet:         gameSet,
		DomainToGameIDs: served,
		UnservedHosts:   unserved,
		Games:           games,
	}
}

func (gr *GameResolver) enabledGames() []string {
	games := gr.GameSet.ToSlice()
	slices.Sort(games)
	return games
}

// cdnGame maps the CDN's game prefix to an enabled title. The host is shared
// by every game, so the path is the only clue: 'athens' is AoM, 'rl' covers
// the other titles and its handler is the same for all of them.
func (gr *GameResolver) cdnGame(urlPath string) (string, bool) {
	if strings.HasPrefix(urlPath, "/aoe/athens-") {
		if gr.GameSet.Contains(commonGame.AoM) {
			return commonGame.AoM, true
		}
		return "", false
	}
	if strings.HasPrefix(urlPath, "/aoe/rl-") {
		for _, gameID := range gr.enabledGames() {
			if gameID != commonGame.AoM {
				return gameID, true
			}
		}
	}
	return "", false
}

func (gr *GameResolver) hostAllowed(host, gameID string) bool {
	games, inMap := gr.DomainToGameIDs[host]
	if !inMap {
		return true
	}
	return slices.Contains(games, gameID)
}

func (gr *GameResolver) isUnserved(host string) bool {
	return gr.UnservedHosts[host]
}

// Resolve returns 0 when the game was resolved, 400 when a game route has no
// resolvable game and 404 when the host is explicitly unserved. bodyGameID is
// the candidate the dispatcher peeked out of an API body, empty when none.
func (gr *GameResolver) Resolve(r *http.Request, bodyGameID string) (gameID string, status int) {
	host := common.NormalizeHost(r.Host)
	if gr.isUnserved(host) {
		return "", http.StatusNotFound
	}
	if title := requestValue(r, "title"); title != "" {
		if !gr.GameSet.Contains(title) || !gr.hostAllowed(host, title) {
			return "", http.StatusBadRequest
		}
		// A session of another title alongside the title is a contradiction:
		// the request says two different games at once.
		if owner, ambiguous := gr.sessionOwner(requestValue(r, "sessionID")); ambiguous || (owner != "" && owner != title) {
			return "", http.StatusBadRequest
		}
		return title, 0
	}
	if sessionID := requestValue(r, "sessionID"); sessionID != "" {
		if found, ambiguous := gr.sessionOwner(sessionID); ambiguous {
			return "", http.StatusBadRequest
		} else if found != "" {
			if !gr.hostAllowed(host, found) {
				return "", http.StatusBadRequest
			}
			return found, 0
		}
	}
	if bodyGameID != "" {
		if !gr.GameSet.Contains(bodyGameID) || !gr.hostAllowed(host, bodyGameID) {
			return "", http.StatusBadRequest
		}
		return bodyGameID, 0
	}
	if host == common.NormalizeHost(common.CdnAgeOfEmpires) && strings.HasPrefix(r.URL.Path, "/aoe/") {
		if gameID, ok := gr.cdnGame(r.URL.Path); ok {
			if !gr.hostAllowed(host, gameID) {
				return "", http.StatusBadRequest
			}
			return gameID, 0
		}
		// An unknown marker is not a rejection by itself: the path falls to
		// the host step below, which serves it when a single game is enabled
		// (the catch-all HEAD proxied) and answers 400 when it is ambiguous.
	}
	if games, inMap := gr.DomainToGameIDs[host]; inMap {
		var enabled []string
		for _, g := range games {
			if gr.GameSet.Contains(g) {
				enabled = append(enabled, g)
			}
		}
		if len(enabled) == 1 {
			return enabled[0], 0
		}
		return "", http.StatusBadRequest
	}
	if enabled := gr.enabledGames(); len(enabled) == 1 {
		return enabled[0], 0
	}
	return "", http.StatusBadRequest
}

// ResolveCloudFiles selects the game whose cloud file credentials own the
// request signature. The upload body is never read here.
func (gr *GameResolver) ResolveCloudFiles(r *http.Request) (string, int) {
	host := common.NormalizeHost(r.Host)
	if gr.isUnserved(host) {
		return "", http.StatusNotFound
	}
	signature := r.URL.Query().Get("sig")
	if signature == "" {
		return "", http.StatusBadRequest
	}
	found := ""
	for _, candidate := range gr.enabledGames() {
		game := gr.Games[candidate]
		if game == nil || game.Resources() == nil {
			continue
		}
		store := game.Resources().CloudFiles()
		if store == nil || !store.Enabled() || store.Credentials == nil {
			continue
		}
		if _, ok := store.Credentials.Get(signature); ok {
			if found != "" {
				return "", http.StatusBadRequest
			}
			found = candidate
		}
	}
	if found == "" {
		return "", http.StatusBadRequest
	}
	if !gr.hostAllowed(host, found) {
		return "", http.StatusBadRequest
	}
	return found, 0
}

// resolveWssToken returns the game a websocket session token belongs to, after
// applying the host rule of the token's game.
func (gr *GameResolver) resolveWssToken(r *http.Request, token string) (models.Session, models.Game, bool) {
	host := common.NormalizeHost(r.Host)
	if gr.isUnserved(host) || token == "" {
		return nil, nil, false
	}
	var foundSession models.Session
	var foundGame models.Game
	for _, candidate := range gr.enabledGames() {
		game := gr.Games[candidate]
		if game == nil || game.Sessions() == nil {
			continue
		}
		session, ok := game.Sessions().GetById(token)
		if !ok {
			continue
		}
		if foundSession != nil || !gr.hostAllowed(host, candidate) {
			return nil, nil, false
		}
		foundSession = session
		foundGame = game
	}
	return foundSession, foundGame, foundSession != nil
}

// Dispatcher picks the mux of a game per request. It never injects the game
// context: TitleMiddleware inside the chosen mux does that, before Playfab, the
// API trees, the CDN, community and the game routes read models.G.
type Dispatcher struct {
	Games    map[string]http.Handler
	Resolver *GameResolver
}

func NewDispatcher(games map[string]http.Handler, resolver *GameResolver) *Dispatcher {
	return &Dispatcher{Games: games, Resolver: resolver}
}

func writeUnavailableGame(w http.ResponseWriter, status int) {
	message := http.StatusText(status)
	if status == http.StatusBadRequest {
		message = "Unavailable game type"
	}
	http.Error(w, message, status)
}

func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := common.NormalizeHost(r.Host)
	if d.Resolver.isUnserved(host) {
		http.NotFound(w, r)
		return
	}
	if d.serveLibraryOps(w, r) {
		return
	}
	urlPath := r.URL.Path
	switch {
	case isWssPath(urlPath):
		d.serveWss(w, r)
		return
	case isUploadPath(urlPath):
		gameID, status := d.Resolver.ResolveCloudFiles(r)
		d.serveGame(w, r, gameID, status)
		return
	}
	if !isGamePath(urlPath) {
		// /, /favicon.ico and any unknown path are not game routes: they are
		// 404, never the 400 a game route without a game answers.
		http.NotFound(w, r)
		return
	}
	var bodyGameID string
	switch {
	case !isAPIPath(urlPath) && needsForm(r):
		r.Body = http.MaxBytesReader(w, r.Body, dispatcherFormLimit)
		if err := r.ParseForm(); err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "Content Too Large", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}
			return
		}
	case isAPIPeekPath(r):
		r.Body = http.MaxBytesReader(w, r.Body, dispatcherPeekLimit)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// A partial read is never served with a truncated body nor with a
			// half-restored one.
			http.Error(w, "Content Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		candidate, conflict := titleFromBody(body)
		if conflict {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		bodyGameID = candidate
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	gameID, status := d.Resolver.Resolve(r, bodyGameID)
	if status == http.StatusBadRequest {
		logRejected(r)
	}
	d.serveGame(w, r, gameID, status)
}

func (d *Dispatcher) serveGame(w http.ResponseWriter, r *http.Request, gameID string, status int) {
	if status != 0 {
		writeUnavailableGame(w, status)
		return
	}
	handler, ok := d.Games[gameID]
	if !ok {
		writeUnavailableGame(w, http.StatusBadRequest)
		return
	}
	handler.ServeHTTP(w, r)
}

// serveLibraryOps sends the library's public diagnostics to an enabled game's
// mux. The library is one instance for the whole process, so any game's handler
// carries it, and its HostMiddleware keeps the paths on the API hosts only.
func (d *Dispatcher) serveLibraryOps(w http.ResponseWriter, r *http.Request) bool {
	if !libraryOpsPaths[r.URL.Path] {
		return false
	}
	games := d.Resolver.enabledGames()
	if len(games) == 0 {
		http.NotFound(w, r)
		return true
	}
	handler, ok := d.Games[games[0]]
	if !ok {
		http.NotFound(w, r)
		return true
	}
	handler.ServeHTTP(w, r)
	return true
}

func (d *Dispatcher) serveWss(w http.ResponseWriter, r *http.Request) {
	wss.HandleWith(w, r, func(token string) (models.Session, models.Game, bool) {
		return d.Resolver.resolveWssToken(r, token)
	})
}

// rejectionLog is the evidence used to fill the shared-host matrix. It is rate
// limited per host and path so a client hammering an unresolvable route cannot
// flood the main log.
var rejectionLog = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

const rejectionLogInterval = 30 * time.Second

func logRejected(r *http.Request) {
	key := common.NormalizeHost(r.Host) + " " + r.URL.Path
	now := time.Now()
	rejectionLog.Lock()
	last, seen := rejectionLog.last[key]
	if seen && now.Sub(last) < rejectionLogInterval {
		rejectionLog.Unlock()
		return
	}
	rejectionLog.last[key] = now
	// Hosts and paths are client controlled, so the map needs a bound. Purging
	// the entries the rate limit no longer needs keeps it small without losing
	// the evidence of the last minutes.
	if len(rejectionLog.last) > 1024 {
		for stale, seenAt := range rejectionLog.last {
			if now.Sub(seenAt) > 5*rejectionLogInterval {
				delete(rejectionLog.last, stale)
			}
		}
	}
	rejectionLog.Unlock()
	logger.Warn("Unavailable game type: host=%s path=%s method=%s user-agent=%q",
		common.NormalizeHost(r.Host), r.URL.Path, r.Method, r.UserAgent())
}
