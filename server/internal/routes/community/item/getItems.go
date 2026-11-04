package item

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
)

// GetItems returns the items list.
// Public endpoint without a game session.
func GetItems(w http.ResponseWriter, r *http.Request) {
	models.G(r).Resources().ReturnSignedAsset("items.json", &w, r, false)
}