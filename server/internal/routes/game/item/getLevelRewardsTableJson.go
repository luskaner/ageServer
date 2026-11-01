package item

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/models"
)

func GetLevelRewardsTableJson(w http.ResponseWriter, r *http.Request) {
	models.G(r).Resources().ReturnSignedAsset("levelRewardsTable.json", &w, r, true)
}
