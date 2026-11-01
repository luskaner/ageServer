package chat

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetChatChannels(w http.ResponseWriter, r *http.Request) {
	i.JSON(&w, i.A{0, models.G(r).ChatChannels().Encode(), 100})
}
