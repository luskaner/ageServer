package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// chatroomRequestID is the part of a chat request that names the channel, so
// bindChatRequest can resolve it without knowing the concrete request type.
type chatroomRequestID interface {
	chatroomID() int32
}

// bindChatRequest binds the request and resolves the chat channel and the
// session's user, writing the error response on failure.
func bindChatRequest[T chatroomRequestID](w http.ResponseWriter, r *http.Request, req *T) (models.Game, models.ChatChannel, models.User, bool) {
	if err := i.Bind(r, req); err != nil {
		i.JSON(&w, i.A{2})
		return nil, nil, nil, false
	}
	game := models.G(r)
	chatChannel, ok := game.ChatChannels().GetById((*req).chatroomID())
	if !ok {
		i.JSON(&w, i.A{2})
		return nil, nil, nil, false
	}
	user, ok := game.Users().GetUserById(models.SessionOrPanic(r).GetUserId())
	if !ok {
		i.JSON(&w, i.A{2})
		return nil, nil, nil, false
	}
	return game, chatChannel, user, true
}
