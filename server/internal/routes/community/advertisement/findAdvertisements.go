package advertisement

import (
	"net/http"
	"sort"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// FindAdvertisements returns visible and joinable advertisements with player profiles.
// Public endpoint without a game session.
func FindAdvertisements(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	profileIDs := make(map[int32]struct{})
	advertisements := g.Advertisements().LockedFindAdvertisementsEncoded(
		g.Title(),
		i.V190,
		0,
		0,
		true,
		func(advertisement models.Advertisement) bool {
			if !advertisement.UnsafeGetVisible() || !advertisement.UnsafeGetJoinable() {
				return false
			}
			_, peerIDs := advertisement.GetPeers().Keys()
			for profileID := range peerIDs {
				profileIDs[profileID] = struct{}{}
			}
			return true
		},
	)
	ids := make([]int32, 0, len(profileIDs))
	for id := range profileIDs {
		ids = append(ids, id)
	}
	ids = normalizeProfileIDs(ids)
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	users := i.A{}
	for _, id := range ids {
		if user, ok := g.Users().GetUserById(id); ok {
			users = append(users, user.EncodeProfileInfo(i.V190))
		}
	}
	i.JSON(&w, i.A{0, advertisements, users})
}

func normalizeProfileIDs(ids []int32) []int32 {
	seen := make(map[int32]struct{}, len(ids))
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}