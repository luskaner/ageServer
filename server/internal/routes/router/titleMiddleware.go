package router

import (
	"context"
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/initializer"
)

func TitleMiddleware(gameId string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), models.GameContextKey, initializer.Games[gameId])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
