package shared

import (
	"net/http"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
)

// MaxQueryItems bounds how many elements one array parameter may contribute and
// MaxQueryString bounds the length of a single profile name or alias. The
// public API is the only unauthenticated surface that takes caller-supplied
// identifiers, so both limits are enforced by dropping the excess instead of
// failing the request (§1.5).
const (
	MaxQueryItems  = 100
	MaxQueryString = 128
)

// ProfileQuery binds every spelling of profile identification the endpoints
// accept, using the same mechanism the /game routes already use: a struct with
// i.Json fields and a schema tag, decoded through i.Bind (see
// routes/game/advertisement/updateTags.go and
// routes/game/leaderboard/getRecentMatchHistory.go).
//
// The spellings differ per endpoint and are not guessable, so they are not
// unified (§3.2): profile_ids (snake), profileids (no separator),
// profile_names (snake), profileNames (camel, Steam proxy only) and aliases.
type ProfileQuery struct {
	ProfileIDs        i.Json[[]int32]  `schema:"profile_ids"`
	ProfileIDsNoSep   i.Json[[]int32]  `schema:"profileids"`
	ProfileNames      i.Json[[]string] `schema:"profile_names"`
	ProfileNamesCamel i.Json[[]string] `schema:"profileNames"`
	Aliases           i.Json[[]string] `schema:"aliases"`
}

// MatchQuery binds the match id array of getMatchHistory and getReplayFiles.
type MatchQuery struct {
	MatchIDs i.Json[[]int64] `schema:"matchIDs"`
}

// BindQuery decodes the query string into dst. A malformed value is soft: the
// affected field keeps its zero value and the rest of the request still binds,
// because nothing in this API fails a request over an unusable parameter
// (§1.5). i.Bind already ignores parameters the struct does not declare.
func BindQuery[D any](r *http.Request, dst *D) {
	_ = i.Bind(r, dst)
}

// ProfileRequest merges the accepted spellings into one request, dropping
// unusable ids and capping the list.
func (q ProfileQuery) ProfileRequest() ProfileRequest {
	ids := append(q.ProfileIDs.Data, q.ProfileIDsNoSep.Data...)
	return ProfileRequest{
		ProfileIDs: normalizeIDs(ids),
		Names:      normalizeStrings(append(q.ProfileNames.Data, q.ProfileNamesCamel.Data...)),
		Aliases:    normalizeStrings(q.Aliases.Data),
	}
}

// IDs returns the usable match ids, in request order.
func (q MatchQuery) IDs() []int64 {
	ids := make([]int64, 0, len(q.MatchIDs.Data))
	for _, id := range q.MatchIDs.Data {
		if id <= 0 {
			continue
		}
		ids = append(ids, id)
		if len(ids) >= MaxQueryItems {
			break
		}
	}
	return ids
}

// normalizeIDs drops the placeholders the client pads with, de-duplicates and
// caps the list, preserving the request order.
func normalizeIDs(ids []int32) []int32 {
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
		if len(out) >= MaxQueryItems {
			break
		}
	}
	return out
}

// normalizeStrings sanitises, de-duplicates and caps a list of names.
func normalizeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = SanitizeString(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= MaxQueryItems {
			break
		}
	}
	return out
}

// SanitizeString trims a caller-supplied name and drops it when it is longer
// than MaxQueryString. Returning "" means "not usable", never an error.
//
// Trimming happens before the length check so padding cannot be used to push a
// usable value over the limit, and an alias that only differs by surrounding
// whitespace still matches the profile it names.
func SanitizeString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > MaxQueryString {
		return ""
	}
	return value
}
