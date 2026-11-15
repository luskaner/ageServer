package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
)

func LeaveChannel(w http.ResponseWriter, r *http.Request) {
	var req chatroomRequest
	game, chatChannel, user, ok := bindChatRequest(w, r, &req)
	if !ok {
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
