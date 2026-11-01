package item

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

type signItemsRequest struct {
	itemRequest
}

// SignItems answers the signature covering the requested items.
//
// The signed payload is the itemIDs value exactly as it arrived, so the client
// verifies against the very string it sent instead of against a re-serialisation
// of the same list. That only holds when this server signed the request, so with
// Sign off there is nothing to verify against and the reply is the same refusal
// the official server gives: result code 2 and no signature.
//
// A list that does not parse is refused the same way: there is no item set to
// speak of, so there is nothing to sign.
//
// If the requesting user's platform user ID is in SignConfigRuntime.PlatformUserIdExceptions,
// the signing behavior is inverted: users in the exception list get the opposite
// of the global Enabled setting.
func SignItems(w http.ResponseWriter, r *http.Request) {
	// Check if this user is in the exception list (reverse behavior)
	invertSigning := isPlatformUserIdException(r)

	// Global Enabled XOR invertSigning = effective signing for this user
	effectiveEnabled := i.SignConfigRuntime.Enabled != invertSigning

	if !effectiveEnabled {
		i.JSON(&w, i.A{2, ""})
		return
	}
	var req signItemsRequest
	if err := i.Bind(r, &req); err != nil {
		i.JSON(&w, i.A{2, ""})
		return
	}
	signature, err := models.SignPayload([]byte(requestedItemIds(r)))
	if err != nil {
		i.JSON(&w, i.A{2, ""})
		return
	}
	i.JSON(&w, i.A{0, signature})
}

// isPlatformUserIdException checks if the requesting user's platform user ID
// is in the SignConfigRuntime.PlatformUserIdExceptions list.
func isPlatformUserIdException(req *http.Request) bool {
	userVal := req.Context().Value("user")
	if userVal == nil {
		return false
	}
	user, ok := userVal.(models.User)
	if !ok {
		return false
	}
	platformUserId := user.GetPlatformUserID()
	for _, exception := range i.SignConfigRuntime.PlatformUserIdExceptions {
		if exception == strconv.FormatUint(platformUserId, 10) {
			return true
		}
	}
	return false
}

// requestedItemIds returns the itemIDs value verbatim, whether it arrived in the
// form body or the query string. i.Bind has already parsed the form by the time
// this runs.
func requestedItemIds(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.Query().Get("itemIDs")
	}
	if value := r.PostForm.Get("itemIDs"); value != "" {
		return value
	}
	return r.URL.Query().Get("itemIDs")
}
