// Package advertisement implements the public lobby listing.
package advertisement

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// Response is the body of findAdvertisements.
type Response struct {
	Result  shared.Result           `json:"result"`
	Matches []models.CommunityMatch `json:"matches"`
	Avatars []*shared.Profile       `json:"avatars"`
}

// FindAdvertisements returns the open lobbies with their members and the profiles
// of those members.
//
// The public API needs an object encoder, not UnsafeEncode: that one is
// positional and its length and fields change per title and client library
// version, so there are no stable indices to map (§4.a). EncodeCommunity lives in
// the models package so it can read the unexported advertisement fields without
// exposing them through a wider interface.
//
// Only waiting, visible lobbies are listed, exactly like the game route's
// "ongoing == false" case. findAdvertisements with zero lobbies is a normal
// answer, not a stub.
func FindAdvertisements(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	response := Response{Result: shared.OK(), Matches: make([]models.CommunityMatch, 0)}
	memberIDs := make([]int32, 0)
	// LockedForEach walks every advertisement under its read lock. The official
	// endpoint is not paginated either, so there is no length/offset here.
	game.Advertisements().LockedForEach(func(adv models.Advertisement) {
		if adv.UnsafeGetState() != 0 || !adv.UnsafeGetVisible() {
			return
		}
		encoded, peers := adv.EncodeCommunity()
		response.Matches = append(response.Matches, encoded)
		memberIDs = append(memberIDs, peers...)
	})
	// avatars[] is the de-duplicated union of every lobby member.
	response.Avatars = shared.ResolveProfiles(r, shared.ProfileRequest{ProfileIDs: memberIDs})
	i.JSON(&w, response)
}
