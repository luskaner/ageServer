package challenge

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/challenge/shared"
)

// GetChallengeProgressByProfileID answers with the progress of the requested
// profile instead of the progress of the caller. A missing profile_id (or an
// unknown profile) is answered with an empty list, as the client does not need
// it to render its own progress.
func GetChallengeProgressByProfileID(w http.ResponseWriter, r *http.Request) {
	rawProfileID := r.URL.Query().Get("profile_id")
	if rawProfileID == "" {
		rawProfileID = r.FormValue("profile_id")
	}
	profileID, err := strconv.ParseInt(rawProfileID, 10, 32)
	if err != nil {
		i.JSON(&w, i.A{0, i.A{}})
		return
	}
	game := models.G(r)
	user, ok := game.Users().GetUserByProfileId(int32(profileID))
	if !ok {
		i.JSON(&w, i.A{0, i.A{}})
		return
	}
	i.JSON(&w, i.A{0, shared.GetChallengeProgressData(user)})
}
