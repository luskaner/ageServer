package community

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/routes/community/achievement"
	"github.com/luskaner/ageServer/server/internal/routes/community/advertisement"
	"github.com/luskaner/ageServer/server/internal/routes/community/clan"
	"github.com/luskaner/ageServer/server/internal/routes/community/communityEvent"
	"github.com/luskaner/ageServer/server/internal/routes/community/item"
	"github.com/luskaner/ageServer/server/internal/routes/community/leaderboard"
	"github.com/luskaner/ageServer/server/internal/routes/community/news"
)

// Register registers all Community API routes.
func Register(group interface {
	HandleFunc(method, path string, handler http.HandlerFunc)
}) {
	group.HandleFunc("GET", "/community/leaderboard/getLeaderBoard2", leaderboard.GetLeaderBoard2)
	group.HandleFunc("GET", "/community/leaderboard/getRecentMatchHistory", leaderboard.GetRecentMatchHistory)
	group.HandleFunc("GET", "/community/leaderboard/getStatGroupsByProfileIDs", leaderboard.GetStatGroupsByProfileIDs)
	group.HandleFunc("GET", "/community/achievement/getAvailableAchievements", achievement.GetAvailableAchievements)
	group.HandleFunc("GET", "/community/achievement/getAchievements", achievement.GetAchievements)
	group.HandleFunc("GET", "/community/clan/find", clan.FindClans)
	group.HandleFunc("GET", "/community/clan/getClanInfoFull", clan.GetClanInfoFull)
	group.HandleFunc("GET", "/community/advertisement/findAdvertisements", advertisement.FindAdvertisements)
	group.HandleFunc("GET", "/community/CommunityEvent/getAvailableCommunityEvents", communityEvent.GetAvailableCommunityEvents)
	group.HandleFunc("GET", "/community/item/getItems", item.GetItems)
	group.HandleFunc("GET", "/community/news/getNews", news.GetNews)
	logger.Printf("Community API routes registered\n")
}