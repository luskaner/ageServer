package router

import (
	"io"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/luskaner/ageServer/server/internal"
)

type ConditionalHandler interface {
	Initializer
	Check(r *http.Request) bool
	Initialize(gameId string) bool
}

func HostMiddleware(gameId string, writer io.Writer) http.Handler {
	var condHandlerOrder = []ConditionalHandler{
		&PlayfabApi{},
		NewApiAgeOfEmpires(),
		NewAoe4ApiAgeOfEmpires(),
		&CdnAgeOfEmpires{},
		&Game{},
	}
	var writers []*internal.PrefixedWriter
	var handls []http.Handler
	var connHandls []ConditionalHandler
	for _, condHandler := range condHandlerOrder {
		if condHandler.Initialize(gameId) {
			handler := condHandler.InitializeRoutes(gameId, nil)
			writers = append(writers, internal.NewPrefixedWriter(writer, gameId, condHandler.Name()))
			connHandls = append(connHandls, condHandler)
			handls = append(handls, handler)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < len(connHandls); i++ {
			if connHandler := connHandls[i]; connHandler.Check(r) {
				next := handls[i]
				writr := writers[i]
				handlers.CustomLoggingHandler(writr, next, logFormatter).ServeHTTP(w, r)
				return
			}
		}
		// Should not arrive here
	})
}
