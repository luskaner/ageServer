package chat

import (
	"net/http"
	"slices"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type textRequest struct {
	Message string `schema:"message"`
}

type sendTextRequest struct {
	chatroomRequest
	textRequest
}

func SendText(w http.ResponseWriter, r *http.Request) {
	var req sendTextRequest
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
	if !ok || !chatChannel.HasUser(user) {
		i.JSON(&w, i.A{2})
		return
	}
	i.JSON(&w, i.A{0})
	gameTitle := game.Title()
	sessions := game.Sessions()
	legacyStaticResponse := i.A{strconv.Itoa(int(req.ChatroomID)), strconv.Itoa(int(user.GetId())), "", req.Message}
	modernStaticResponse := append(slices.Clone(legacyStaticResponse), "", req.Message)
	for existingUser := range chatChannel.GetUsers() {
		var existingUserSession models.Session
		existingUserSession, ok = sessions.GetByUserId(existingUser.GetId())
		var msg i.A
		if i.SinceTheBalticPowers(gameTitle, existingUserSession.GetClientLibVersion()) {
			msg = modernStaticResponse
		} else {
			msg = legacyStaticResponse
		}
		wss.SendOrStoreMessage(
			existingUserSession,
			"ChannelChatMessage",
			msg,
		)
	}
}
