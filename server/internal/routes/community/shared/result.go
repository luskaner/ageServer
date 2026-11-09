// Package shared holds the pieces every /community endpoint needs: the result
// envelope, the object types of the responses, the encoders for profiles and
// ladder rows, input parsing and the stub helper.
//
// The /community API is the object-shaped counterpart of /game: the game client
// parses positional arrays, while this API (documented for LibreMatch and used
// by third-party tools) returns JSON objects with named fields. Both encodings
// therefore coexist on purpose; see docs/community-api-implementation.md §D1.
//
// Responses are typed structs rather than maps so a field rename is a compile
// error instead of a silent contract break. The few places that stay map-based
// are the ones whose JSON really is keyed by caller-supplied ids
// (userAchievementsMap, itemInstancesByProfileID).
package shared

// Result codes of the public API. Code 0 is success; 2 is the value the
// official service uses for an invalid request, and what this server returns
// for an unusable title (§D4) instead of the official 404.
const (
	CodeOK      = 0
	CodeInvalid = 2
)

// Result is the envelope every endpoint wraps its payload in:
// {"result":{"code":0,"message":"SUCCESS"}}.
type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// OK is the success envelope.
func OK() Result {
	return Result{Code: CodeOK, Message: "SUCCESS"}
}

// Err is the failure envelope.
func Err() Result {
	return Result{Code: CodeInvalid, Message: "INVALID_REQUEST"}
}

// ErrorResponse is the body of a request rejected before reaching a handler,
// which today means an unusable title (§D4). It carries the envelope only: the
// official service answers 404 with no body at all, and §0.3's shape rule covers
// stubs that answer successfully, not errors.
type ErrorResponse struct {
	Result Result `json:"result"`
}

// Error builds the rejection body.
func Error() ErrorResponse {
	return ErrorResponse{Result: Err()}
}
