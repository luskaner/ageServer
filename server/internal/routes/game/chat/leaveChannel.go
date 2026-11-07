package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func LeaveChannel(w http.ResponseWriter, r *http.Request) {
	var req chatroomRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	game := models.G(r)
	chatChannel, ok := game.ChatChannels().GetById(req.ChatroomID)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	sess := models.SessionOrPanic(r)
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	existed, members := chatChannel.RemoveUser(user)
	if !existed {
		i.JSON(&w, i.A{2})
		return
	}
	i.JSON(&w, i.A{0})
	// Only the members that are left are told: the ones that join later never
	// have this user in their roster.
	NotifyLeaveChannel(game.Sessions(), members, user, chatChannel.GetId())
}
