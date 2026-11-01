package chat

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

func NotifyLeaveChannel(sessions models.Sessions, users models.Users, user models.User, chatChannelId int32, clientLibVersion uint16) {
	staticResponse := i.A{strconv.Itoa(int(chatChannelId)), user.EncodeProfileInfo(clientLibVersion)}
	for userId := range users.GetUserIds() {
		if userId == user.GetId() {
			continue
		}
		existingUserSession, ok := sessions.GetByUserId(userId)
		if ok {
			wss.SendOrStoreMessage(
				existingUserSession,
				"ChannelLeaveMessage",
				staticResponse,
			)
		}
	}
}

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
	users := game.Users()
	user, ok := users.GetUserById(sess.GetUserId())
	if !ok || !chatChannel.RemoveUser(user) {
		i.JSON(&w, i.A{2})
		return
	}
	i.JSON(&w, i.A{0})
	NotifyLeaveChannel(game.Sessions(), users, user, chatChannel.GetId(), sess.GetClientLibVersion())
}
