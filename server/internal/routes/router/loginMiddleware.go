package router

import (
	"context"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/login"
)

func LoginUserMiddleware(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now().UTC()
		var req login.PlatformLoginRequest
		if err := i.Bind(r, &req); err != nil {
			login.PlatformLoginError(t, w)
			return
		}
		g := models.G(r)
		title := g.Title()
		var avatarStatDefinitions models.AvatarStatDefinitions = nil
		if title != game.AoE1 {
			avatarStatDefinitions = g.LeaderboardDefinitions().AvatarStatDefinitions()
		}
		u := g.Users().GetOrCreateUser(
			title,
			g.Items(),
			avatarStatDefinitions,
			r.RemoteAddr,
			req.MacAddress,
			req.AccountType == "XBOXLIVE",
			req.PlatformUserId,
			req.Alias,
		)
		ctx := context.WithValue(r.Context(), "user", u)
		ctx = context.WithValue(ctx, "request", req)
		ctx = context.WithValue(ctx, "time", t)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
