package party

import (
	"net/http"
	"slices"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type request struct {
	MessageTypeID uint8  `schema:"messageTypeID"`
	MatchID       int32  `schema:"match_id"`
	Broadcast     bool   `schema:"broadcast"`
	Message       string `schema:"message"`
	Metadata      string `schema:"metadata"`
}

type profileIds struct {
	Ids i.Json[[]int32] `schema:"to_profile_ids"`
}

type profileId struct {
	Id int32 `schema:"to_profile_id"`
}

func SendMatchChat(w http.ResponseWriter, r *http.Request) {
	// FIXME: AoE3 duplicate messages/wrong total
	var req request
	if err := i.Bind(r, &req); err != nil {
		i.JSON(&w, i.A{2})
		return
	}

	var toProfileIds profileIds
	g := models.G(r)
	session := models.SessionOrPanic(r)
	if i.BeforeTheBalticPowers(g.Title(), session.GetClientLibVersion()) {
		var toProfileId profileId
		if err := i.Bind(r, &toProfileId); err != nil {
			i.JSON(&w, i.A{2})
			return
		}
		if toProfileId.Id != 0 {
			toProfileIds.Ids.Data = append(toProfileIds.Ids.Data, toProfileId.Id)
		}
	} else if err := i.Bind(r, &toProfileIds); err != nil {
		i.JSON(&w, i.A{2})
		return
	}

	adv, ok := g.Advertisements().GetAdvertisement(req.MatchID)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}

	sess := models.SessionOrPanic(r)
	currentUserId := sess.GetUserId()
	peers := adv.GetPeers()

	// Only peers within the match can send messages
	if _, ok = peers.Load(currentUserId); !ok {
		i.JSON(&w, i.A{2})
		return
	}

	users := g.Users()
	if g.Title() == game.AoM {
		toProfileIds.Ids.Data = slices.DeleteFunc(toProfileIds.Ids.Data, func(id int32) bool { return id == currentUserId })
	}
	receivers := make([]models.User, len(toProfileIds.Ids.Data))

	for j, profId := range toProfileIds.Ids.Data {
		receivers[j], ok = users.GetUserById(profId)
		if !ok {
			i.JSON(&w, i.A{2})
			return
		}
	}

	currentUser, ok := g.Users().GetUserById(currentUserId)
	if !ok {
		i.JSON(&w, i.A{2})
		return
	}
	message := adv.MakeMessage(
		req.Broadcast,
		req.Message,
		req.MessageTypeID,
		req.Metadata,
		currentUser,
		receivers,
	)

	sessions := g.Sessions()
	var receiverSession models.Session
	for _, receiver := range receivers {
		receiverSession, ok = sessions.GetByUserId(receiver.GetId())
		if !ok {
			continue
		}
		wss.SendOrStoreMessage(
			receiverSession,
			"MatchReceivedChatMessage",
			message.Encode(receiverSession.GetClientLibVersion()),
		)
	}
	i.JSON(&w, i.A{0})
}
