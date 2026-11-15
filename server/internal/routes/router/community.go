package router

import (
	"net/http"
	"strings"

	"github.com/luskaner/ageServer/server/internal/routes/community/achievement"
	"github.com/luskaner/ageServer/server/internal/routes/community/advertisement"
	"github.com/luskaner/ageServer/server/internal/routes/community/clan"
	"github.com/luskaner/ageServer/server/internal/routes/community/communityEvent"
	"github.com/luskaner/ageServer/server/internal/routes/community/external"
	"github.com/luskaner/ageServer/server/internal/routes/community/item"
	"github.com/luskaner/ageServer/server/internal/routes/community/leaderboard"
	"github.com/luskaner/ageServer/server/internal/routes/community/news"
)

// Community is the conditional handler that owns the public /community API.
//
// It lives in this package, next to Game, for the same reason Game does: the
// handler tree of a title has to be assembled where the conditional handlers are
// declared. The handlers themselves live under routes/community and are the
// object-shaped counterpart of routes/game.
//
// The API is unauthenticated: the official /community service is public and the
// client sends no sessionID, so SessionMiddleware is deliberately not applied
// here (§1.1, §D2).
type Community struct {
	Router
}

func (c *Community) Name() string {
	return "community"
}

// Check matches the exact prefix. Using a plain HasPrefix (rather than a
// pattern) keeps the rule auditable: everything under /community/ is public,
// and nothing outside it is.
func (c *Community) Check(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/community/")
}

// Initialize runs for every title: the public API is served for all of them,
// with the endpoints it cannot fill answering as stubs.
func (c *Community) Initialize(_ string) bool {
	return true
}

// InitializeRoutes registers the full route map, including the endpoints that
// only have a stub behind them, so a documented path never answers 404 (§D0).
//
// Several paths are registered twice because the OpenAPI spec and the live
// service disagree on capitalisation; both spellings reach the same handler.
func (c *Community) InitializeRoutes(_ string, _ http.Handler) http.Handler {
	c.initialize()
	communityGroup := c.group.Subgroup("/community")

	newsGroup := communityGroup.Subgroup("/news")
	newsGroup.HandleFunc("GET", "/getNews", news.GetNews)

	leaderboardGroup := communityGroup.Subgroup("/leaderboard")
	leaderboardGroup.HandleFunc("GET", "/getAvailableLeaderboards", leaderboard.GetAvailableLeaderboards)
	leaderboardGroup.HandleFunc("GET", "/GetAvailableLeaderboards", leaderboard.GetAvailableLeaderboards)
	leaderboardGroup.HandleFunc("GET", "/getLeaderBoard2", leaderboard.GetLeaderBoard2)
	// GetPersonalStat is capitalised in the spec and the live service while its
	// siblings are lowercase; both spellings are served because the wiki and the
	// spec disagree on this one (§2.b, C3).
	leaderboardGroup.HandleFunc("GET", "/GetPersonalStat", leaderboard.GetPersonalStat)
	leaderboardGroup.HandleFunc("GET", "/getPersonalStat", leaderboard.GetPersonalStat)
	leaderboardGroup.HandleFunc("GET", "/GetAvatarStatForProfile", leaderboard.GetAvatarStatForProfile)
	leaderboardGroup.HandleFunc("GET", "/getRecentMatchHistory", leaderboard.GetRecentMatchHistory)
	leaderboardGroup.HandleFunc("GET", "/getMatchHistory", leaderboard.GetMatchHistory)
	leaderboardGroup.HandleFunc("GET", "/getReplayFiles", leaderboard.GetReplayFiles)
	leaderboardGroup.HandleFunc("GET", "/getStatGroupsByProfileIDs", leaderboard.GetStatGroupsByProfileIDs)

	achievementGroup := communityGroup.Subgroup("/achievement")
	achievementGroup.HandleFunc("GET", "/getAvailableAchievements", achievement.GetAvailableAchievements)
	achievementGroup.HandleFunc("GET", "/getAchievements", achievement.GetAchievements)

	clanGroup := communityGroup.Subgroup("/clan")
	clanGroup.HandleFunc("GET", "/find", clan.Find)
	clanGroup.HandleFunc("GET", "/getClanInfoFull", clan.GetClanInfoFull)

	// CommunityEvent is capitalised in the official path. No lowercase alias is
	// served: the live service only answers the capitalised spelling, so an
	// extra route would be untested surface without evidence behind it.
	communityEventGroup := communityGroup.Subgroup("/CommunityEvent")
	communityEventGroup.HandleFunc("GET", "/getAvailableCommunityEvents", communityEvent.GetAvailableCommunityEvents)

	communityGroup.HandleFunc("GET", "/advertisement/findAdvertisements", advertisement.FindAdvertisements)
	communityGroup.HandleFunc("GET", "/external/proxysteamuserrequest", external.ProxySteamUserRequest)
	communityGroup.HandleFunc("GET", "/item/getInventoryByProfileIDs", item.GetInventoryByProfileIDs)
	communityGroup.HandleFunc("GET", "/item/getItems", item.GetItems)

	// The title is resolved once for the whole tree, so handlers just call
	// models.G(r) like a /game handler does. The bare mux is returned wrapped in
	// that middleware on purpose: SessionMiddleware is deliberately not applied
	// to this tree, and Check already scoped the prefix.
	return CommunityGameMiddleware(c.group.mux)
}
