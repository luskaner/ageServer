package achievement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetAchievements(w http.ResponseWriter, r *http.Request) {
	sess := models.SessionOrPanic(r)
	i.JSON(&w,
		i.A{
			0,
			i.A{
				i.A{
					sess.GetUserId(),
					// DO NOT RETURN ACHIEVEMENTS AS IT WILL *REALLY* GRANT THEM ON XBOX
					i.A{},
				},
			},
		},
	)
}
