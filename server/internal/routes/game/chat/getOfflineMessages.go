package chat

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetOfflineMessages(w http.ResponseWriter, r *http.Request) {
	// Only AoE3 has offline messages but are not implemented as we would need to store the chat messages and who
	// joined the chat channels
	sess := models.SessionOrPanic(r)
	i.JSON(&w, i.A{0, i.A{}, i.A{i.A{strconv.Itoa(int(sess.GetUserId())), i.A{}}}, i.A{}, i.A{}, i.A{}})
}
