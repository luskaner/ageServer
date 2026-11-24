package achievement

import (
	"net/http"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func GetAchievements(w http.ResponseWriter, r *http.Request) {
	sess := models.SessionOrPanic(r)
	earned := i.A{}
	if !models.SafeForRequest(r) {
		earned = allAchievements(models.G(r).Resources())
	}
	i.JSON(&w,
		i.A{
			0,
			i.A{
				i.A{
					sess.GetUserId(),
					earned,
				},
			},
		},
	)
}

// allAchievements returns every non-deleted achievement of the catalogue as
// [[achievementID, achievedDate], ...]. The date is the catalogue's
// dateVisible, which is stable across requests; without one the current time
// is used so the client still accepts the entry.
func allAchievements(resources models.Resources) i.A {
	if resources == nil {
		return i.A{}
	}
	root := resources.ArrayFiles()["achievements.json"]
	if len(root) < 2 {
		return i.A{}
	}
	rows, ok := root[1].(i.A)
	if !ok {
		return i.A{}
	}
	now := time.Now().Unix()
	earned := make(i.A, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(i.A)
		if !ok || len(row) < 8 {
			continue
		}
		if deleted, ok := i.NumberToInt64(row[6]); ok && deleted != 0 {
			continue
		}
		achievementID, ok := i.NumberToInt64(row[0])
		if !ok {
			continue
		}
		date, ok := i.NumberToInt64(row[5])
		if !ok || date <= 0 {
			date = now
		}
		earned = append(earned, i.A{achievementID, date})
	}
	return earned
}
