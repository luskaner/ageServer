package router

import (
	"io"
	"net/http"
	"runtime"

	"github.com/gorilla/handlers"
	"github.com/luskaner/ageServer/common"
	cacertPem "github.com/luskaner/ageServer/server/internal/routes/cacert.pem"
	"github.com/luskaner/ageServer/server/internal/routes/shutdown"
	"github.com/luskaner/ageServer/server/internal/routes/sigkeyder"
	"github.com/luskaner/ageServer/server/internal/routes/test"
)

// RootRoutes builds the routes of a listener: the announce probe, the
// certificate of the listener's mode, the signing key and shutdown, plus the
// dispatcher for everything else. The game is resolved per request, so these
// routes have no game of their own; /test/root uses the same resolver as the
// dispatcher, including its 404 for an unserved host and its 400 when a
// multi-game listener is asked without a title.
func RootRoutes(mode common.CertMode, writer io.Writer, dispatcher http.Handler,
	resolve func(*http.Request, string) (string, int)) http.Handler {

	mux := http.NewServeMux()
	wrap := func(handler http.Handler) http.Handler {
		if writer == nil {
			return handler
		}
		return handlers.CustomLoggingHandler(writer, handler, logFormatter)
	}
	mux.Handle("GET /"+common.AnnouncePath, wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gameID, status := resolve(r, "")
		if status != 0 {
			writeUnavailableGame(w, status)
			return
		}
		test.Test(w, r, gameID, mode)
	})))
	mux.Handle("GET /"+common.CACertPath, wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cacertPem.CacertPem(w, r, mode)
	})))
	mux.Handle("GET /sigkey.der", wrap(http.HandlerFunc(sigkeyder.SigkeyDer)))
	if runtime.GOOS == "windows" {
		mux.Handle("POST /shutdown", wrap(http.HandlerFunc(shutdown.Shutdown)))
	}
	mux.Handle("/", dispatcher)
	return mux
}
