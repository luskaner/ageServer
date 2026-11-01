package relationship

import (
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func relationshipResponse(errorCode int, friends []i.A, lastConnection []i.A) i.A {
	return i.A{
		errorCode,
		friends,
		i.A{},
		i.A{},
		i.A{},
		lastConnection,
		i.A{},
		i.A{},
	}
}

func Relationships(gameTitle string, clientLibVersion uint16, users models.Users, user models.User, presences models.PresenceDefinitions) i.A {
	profileInfo := users.EncodeProfileInfo(presences, func(u models.User) bool {
		return u != user && u.GetPresence() > 0
	}, clientLibVersion)
	friends := profileInfo
	lastConnection := profileInfo
	if gameTitle == game.AoE3 || gameTitle == game.AoE4 || gameTitle == game.AoM {
		lastConnection = []i.A{}
	} else {
		friends = []i.A{}
	}
	return relationshipResponse(0, friends, lastConnection)
}

func GetRelationships(w http.ResponseWriter, r *http.Request) {
	// As we don't have knowledge of Steam/Xbox friends, nor it is supposed to be many players on the server
	// just return all online users as if they were friends (AoE3/AoE4/AoM) or last connections (AoE2)
	sess := models.SessionOrPanic(r)
	g := models.G(r)
	users := g.Users()
	currentUser, ok := users.GetUserById(sess.GetUserId())
	if ok {
		i.JSON(&w, Relationships(g.Title(), sess.GetClientLibVersion(), users, currentUser, g.PresenceDefinitions()))
	} else {
		i.JSON(&w, relationshipResponse(0, []i.A{}, []i.A{}))
	}
}
