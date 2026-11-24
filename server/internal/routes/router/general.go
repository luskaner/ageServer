package router

import (
	"io"
	"net/http"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models/initializer"
)

// General is the single-game form of the listener routes. It is deprecated for
// the server itself, which builds RootRoutes once per listener and dispatches
// per request; it survives as a composed handler for tests that exercise the
// root routes without starting a listener.
type General struct {
	Router
	Writer io.Writer
}

func (g *General) Name() string {
	return "general"
}

func (g *General) InitializeRoutes(gameId string, next http.Handler) http.Handler {
	g.initialize()
	writer := internal.NewPrefixedWriter(g.Writer, gameId, g.Name())
	resolver := NewGameResolver(mapset.NewThreadUnsafeSet[string](gameId), true, initializer.Games)
	return RootRoutes(common.GameCertMode(gameId), writer, next, resolver.Resolve)
}
