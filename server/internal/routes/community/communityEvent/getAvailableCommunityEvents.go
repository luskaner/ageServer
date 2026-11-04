package communityEvent

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
)

// GetAvailableCommunityEvents returns available community events.
// Public endpoint without a game session.
func GetAvailableCommunityEvents(w http.ResponseWriter, r *http.Request) {
	models.G(r).Resources().ReturnSignedAsset("communityEvents.json", &w, r, false)
}