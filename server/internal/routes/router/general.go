package router

import (
	"io"
	"net/http"
	"runtime"

	"github.com/gorilla/handlers"
	"github.com/luskaner/ageServer/server/internal"
	cacertPem "github.com/luskaner/ageServer/server/internal/routes/cacert.pem"
	sigkeyder "github.com/luskaner/ageServer/server/internal/routes/sigkeyder"
	"github.com/luskaner/ageServer/server/internal/routes/shutdown"
	"github.com/luskaner/ageServer/server/internal/routes/test"
)

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
	g.group.Handle(
		"GET",
		"/test",
		handlers.CustomLoggingHandler(writer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			test.Test(w, r)
		}), logFormatter),
	)
	g.group.Handle(
		"GET",
		"/cacert.pem",
		handlers.CustomLoggingHandler(writer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cacertPem.CacertPem(w, r)
		}), logFormatter),
	)
	g.group.Handle(
		"GET",
		"/sigkey.der",
		handlers.CustomLoggingHandler(writer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sigkeyder.SigkeyDer(w, r)
		}), logFormatter),
	)
	if runtime.GOOS == "windows" {
		g.group.Handle(
			"POST",
			"/shutdown",
			handlers.CustomLoggingHandler(writer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				shutdown.Shutdown(w, r)
			}), logFormatter),
		)
	}
	g.group.HandlePath("/", next)
	return g.group.mux
}
