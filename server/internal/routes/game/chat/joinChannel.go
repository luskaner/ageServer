package chat

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
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
	users := game.Users()
	user, ok := users.GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	exists, encodedUsers := chatChannel.AddUser(user, sess.GetClientLibVersion())
	if exists {
		i.JSON(&w, i.A{2, "", 0, i.A{}})
		return
	}
	chatChannelIdStr := strconv.Itoa(int(req.ChatroomID))
	i.JSON(&w, i.A{0, chatChannelIdStr, 0, encodedUsers})
	sessions := game.Sessions()
	staticResponse := i.A{chatChannelIdStr, i.A{0, user.EncodeProfileInfo(sess.GetClientLibVersion())}}
	for userId := range users.GetUserIds() {
		// Should help avoid wrong counters in some cases
		if userId == user.GetId() {
			continue
		}
		var existingUserSession models.Session
		existingUserSession, ok = sessions.GetByUserId(userId)
		if ok {
			wss.SendOrStoreMessage(
				existingUserSession,
				"ChannelJoinMessage",
				staticResponse,
			)
		}
	}
}
