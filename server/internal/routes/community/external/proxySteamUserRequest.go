// Package external implements the public Steam-proxy endpoint.
package external

import (
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// SteamPlayer is one entry of the Steam Web API player list. The fields are
// declared from the live response so a caller can rely on the shape once the
// proxy is implemented; nothing fills it today.
type SteamPlayer struct {
	SteamID             string `json:"steamid"`
	CommunityVisibility int    `json:"communityvisibilitystate"`
	ProfileState        int    `json:"profilestate"`
	PersonaName         string `json:"personaname"`
	ProfileURL          string `json:"profileurl"`
	Avatar              string `json:"avatar"`
	AvatarMedium        string `json:"avatarmedium"`
	AvatarFull          string `json:"avatarfull"`
	AvatarHash          string `json:"avatarhash"`
	PersonaState        int    `json:"personastate"`
	RealName            string `json:"realname,omitempty"`
	PrimaryClanID       string `json:"primaryclanid,omitempty"`
	TimeCreated         int64  `json:"timecreated,omitempty"`
	PersonaStateFlags   int    `json:"personastateflags"`
	LocationCountryCode string `json:"loccountrycode,omitempty"`
}

// SteamResponse is the Steam Web API envelope.
type SteamResponse struct {
	Response struct {
		Players []SteamPlayer `json:"players"`
	} `json:"response"`
}

// Response is the body of proxysteamuserrequest.
type Response struct {
	Result       shared.Result     `json:"result"`
	Avatars      []*shared.Profile `json:"avatars"`
	SteamResults SteamResponse     `json:"steamResults"`
}

// ProxySteamUserRequest returns the profiles of the requested users plus the
// Steam player summaries the official service relays.
//
// This server has no Steam Web API key and should not ship one, so steamResults
// is served empty. The avatars[] side is real, which is the part a caller can
// actually use (§3.c, §8.3).
//
// SECURITY: the `request` parameter is the Steam API path the official service
// concatenates onto its own base URL. It is deliberately not implemented. If the
// outbound proxy is ever added it MUST validate the value against an allowlist of
// /ISteamUser/... paths, or it becomes a server-side request forgery hole (§1.5,
// §3.c).
//
// FIXME(steam-proxy): allowlist `request` against ^/ISteam[A-Za-z]+/[A-Za-z]+/$ if
// the outgoing call is ever made.
func ProxySteamUserRequest(w http.ResponseWriter, r *http.Request) {
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	response := Response{
		Result:  shared.OK(),
		Avatars: shared.ResolveProfiles(r, query.ProfileRequest()),
	}
	response.SteamResults.Response.Players = []SteamPlayer{}
	i.JSON(&w, response)
}
