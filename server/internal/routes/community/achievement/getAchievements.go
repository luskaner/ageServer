package achievement

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
)

// GetAchievements returns achievements for the given profile IDs.
// Public endpoint without a game session.
func GetAchievements(w http.ResponseWriter, r *http.Request) {
	ids := parseProfileIDs(r, "profileids")
	entries := make(i.A, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, i.A{id, i.A{}})
	}
	i.JSON(&w, i.A{0, entries})
}

func parseProfileIDs(r *http.Request, key string) []int32 {
	var ids []int32
	for _, value := range r.URL.Query()[key] {
		id, err := strconv.ParseInt(value, 10, 32)
		if err == nil && id > 0 {
			ids = append(ids, int32(id))
		}
	}
	return ids
}