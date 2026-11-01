package relationship

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

func ChangePresence(clientLibVersion uint16, sessions models.Sessions, users models.Users, user models.User, presenceDefinitions models.PresenceDefinitions, presence int32) {
	user.SetPresence(presence)
	NotifyChangePresence(clientLibVersion, sessions, users, user, presenceDefinitions)
}

func NotifyChangePresence(clientLibVersion uint16, sessions models.Sessions, users models.Users, user models.User, presenceDefinitions models.PresenceDefinitions) {
	profileInfo := i.A{user.EncodeProfileInfo(clientLibVersion)}
	profileInfo[0] = append(profileInfo[0].(i.A), user.EncodePresence(presenceDefinitions)...)
	for u := range users.GetUserIds() {
		sess, ok := sessions.GetByUserId(u)
		if ok {
			wss.SendOrStoreMessage(
				sess,
				"PresenceMessage",
				profileInfo,
			)
		}
	}
}

type setPresenceRequest struct {
	PresenceId int32 `schema:"presence_id"`
}

func SetPresence(w http.ResponseWriter, r *http.Request) {
	var req setPresenceRequest
	if err := i.Bind(r, &req); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	sess := models.SessionOrPanic(r)
	game := models.G(r)
	users := game.Users()
	u, _ := users.GetUserById(sess.GetUserId())
	ChangePresence(sess.GetClientLibVersion(), game.Sessions(), users, u, game.PresenceDefinitions(), req.PresenceId)
	i.JSON(&w, i.A{0})
}
