package chat

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type chatroomRequest struct {
	ChatroomID int32 `schema:"chatroomID"`
}

func JoinChannel(w http.ResponseWriter, r *http.Request) {
	var req chatroomRequest
	err := i.Bind(r, &req)
	if err != nil {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	game := models.G(r)
	chatChannel, ok := game.ChatChannels().GetById(req.ChatroomID)
	if !ok {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	sess := models.SessionOrPanic(r)
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	exists, roster, members := chatChannel.AddUser(user, sess.GetClientLibVersion())
	if exists {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	chatChannelIdStr := strconv.Itoa(int(req.ChatroomID))
	i.JSON(&w, i.A{0, chatChannelIdStr, 0, roster})
	// Only the members that were already inside are told: the ones that join
	// later get this user in the roster they receive.
	NotifyJoinChannel(game.Sessions(), members, user, chatChannelIdStr)
}
