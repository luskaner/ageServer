package models

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

// SafeForRequest reports whether [Safe] applies to this request. It resolves
// the platform user from the login context or, for the routes that do not set
// it, from the session. Requests without a session (the public /community API)
// are never safe.
func SafeForRequest(r *http.Request) bool {
	sess, ok := session(r)
	if !ok {
		return false
	}
	return i.SafeEnabled(sess.Official(), PlatformUserIdForRequest(r))
}

// PlatformUserIdForRequest returns the platform user ID of the requesting
// user. LoginUserMiddleware only runs on platformlogin, so a normal game
// request carries the session and the user is resolved from it.
func PlatformUserIdForRequest(r *http.Request) uint64 {
	if user, ok := r.Context().Value("user").(User); ok && user != nil {
		return user.GetPlatformUserID()
	}
	sess, ok := session(r)
	if !ok {
		return 0
	}
	game, ok := r.Context().Value(GameContextKey).(Game)
	if !ok || game == nil {
		return 0
	}
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok {
		return 0
	}
	return user.GetPlatformUserID()
}
