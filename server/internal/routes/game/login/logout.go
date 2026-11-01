package login

import (
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/chat"
	"github.com/luskaner/ageServer/server/internal/routes/game/relationship"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

func Logout(w http.ResponseWriter, r *http.Request) {
	sess := models.SessionOrPanic(r)
	g := models.G(r)
	users := g.Users()
	advertisements := g.Advertisements()
	u, ok := users.GetUserById(sess.GetUserId())
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	if adv := advertisements.GetUserAdvertisement(u.GetId()); adv != nil {
		advertisements.WithWriteLock(adv.GetId(), func() {
			g.Advertisements().UnsafeRemovePeer(adv.GetId(), u.GetId())
		})
	}
	g.Matchmaking().RemoveUser(u.GetId())
	sessions := g.Sessions()
	for channelId, channel := range g.ChatChannels().Iter() {
		if channel.RemoveUser(u) {
			chat.NotifyLeaveChannel(sessions, users, u, channelId, sess.GetClientLibVersion())
			// AoE3 only takes into account the first notify in a readSession (legacy) return
			// so delay each message by 100ms so they go in different responses
			// otherwise, it would appear as it left the first channel only
			time.Sleep(100 * time.Millisecond)
		}
	}
	relationship.ChangePresence(sess.GetClientLibVersion(), sessions, users, u, g.PresenceDefinitions(), 0)
	if title := g.Title(); title == game.AoE3 || title == game.AoE4 || title == game.AoM {
		profileInfo := u.EncodeProfileInfo(sess.GetClientLibVersion())
		for user := range users.GetUserIds() {
			if user != u.GetId() {
				currentSess, currentOk := sessions.GetByUserId(user)
				if currentOk {
					wss.SendOrStoreMessage(
						currentSess,
						"FriendClearMessage",
						i.A{profileInfo, 0},
					)
				}
			}
		}
	}
	sessions.Delete(sess.Id())
	i.JSON(&w, i.A{0})
}
