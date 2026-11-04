package news

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
)

// GetNews returns the news feed.
// Public endpoint without a game session.
func GetNews(w http.ResponseWriter, r *http.Request) {
	models.G(r).Resources().ReturnSignedAsset("news.json", &w, r, false)
}