package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type recipientIDs struct {
	IDs i.Json[[]int32] `schema:"recipientIDs"`
}

type recipientID struct {
	ID int32 `schema:"recipientID"`
}

func whisperResult(w *http.ResponseWriter, legacy bool, code int) {
	response := i.A{code} // resultCode
	if !legacy {
		// FIXME: Is it the 0 repeated for each recipient?
		response = append(response, i.A{0}) // per-recipient resultCode
	}
	i.JSON(w, response)
}

func SendWhisper(w http.ResponseWriter, r *http.Request, legacy bool) {
	g := models.G(r)
	var req textRequest
	err := i.Bind(r, &req)
	if err != nil {
		whisperResult(&w, legacy, 2)
		return
	}

	var targetUserIds recipientIDs
	if !legacy {
		if err := i.Bind(r, &targetUserIds); err != nil {
			whisperResult(&w, legacy, 2)
			return
		}
	} else {
		var recpId recipientID
		if err = i.Bind(r, &recpId); err != nil {
			whisperResult(&w, legacy, 2)
			return
		}
		targetUserIds.IDs.Data = append(targetUserIds.IDs.Data, recpId.ID)
	}
	users := g.Users()
	receivers := make([]models.User, len(targetUserIds.IDs.Data))
	var ok bool

	for j, profileId := range targetUserIds.IDs.Data {
		receivers[j], ok = users.GetUserById(profileId)
		if !ok {
			whisperResult(&w, legacy, 2)
			return
		}
	}
	currentSession := models.SessionOrPanic(r)
	currentUser, ok := users.GetUserById(currentSession.GetUserId())
	if !ok {
		whisperResult(&w, legacy, 2)
		return
	}

	// message structure: [empty, [senderName, message], message]
	message := i.A{""}
	if !legacy {
		message = append(
			message,
			i.A{
				"",          // senderName (empty)
				req.Message, // message
			}...,
		)
	}
	message = append(message, req.Message) // message
	sessions := g.Sessions()
	var receiverSession models.Session
	for _, receiver := range receivers {
		receiverSession, ok = sessions.GetByUserId(receiver.GetId())
		if !ok {
			continue
		}
		finalMessage := i.A{
			currentUser.EncodeProfileInfo(receiverSession.GetClientLibVersion()),
		}
		finalMessage = append(finalMessage, message...)
		wss.SendOrStoreMessage(
			receiverSession,
			"PersonalChatMessage",
			finalMessage,
		)
	}

	whisperResult(&w, legacy, 0)
}
