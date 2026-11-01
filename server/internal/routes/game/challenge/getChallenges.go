package challenge

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
)

func GetChallenges(w http.ResponseWriter, r *http.Request) {
	models.G(r).Resources().ReturnSignedAsset("challenges.json", &w, r, false)
}
