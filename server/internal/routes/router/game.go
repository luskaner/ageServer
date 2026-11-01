package router

import (
	"net/http"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/cloudfiles"
	"github.com/luskaner/ageServer/server/internal/routes/game/account"
	"github.com/luskaner/ageServer/server/internal/routes/game/achievement"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement"
	Automatch2 "github.com/luskaner/ageServer/server/internal/routes/game/automatch2"
	"github.com/luskaner/ageServer/server/internal/routes/game/challenge"
	"github.com/luskaner/ageServer/server/internal/routes/game/chat"
	"github.com/luskaner/ageServer/server/internal/routes/game/clan"
	"github.com/luskaner/ageServer/server/internal/routes/game/cloud"
	"github.com/luskaner/ageServer/server/internal/routes/game/communityEvent"
	"github.com/luskaner/ageServer/server/internal/routes/game/invitation"
	"github.com/luskaner/ageServer/server/internal/routes/game/item"
	"github.com/luskaner/ageServer/server/internal/routes/game/leaderboard"
	"github.com/luskaner/ageServer/server/internal/routes/game/login"
	"github.com/luskaner/ageServer/server/internal/routes/game/news"
	"github.com/luskaner/ageServer/server/internal/routes/game/party"
	"github.com/luskaner/ageServer/server/internal/routes/game/playerreport"
	"github.com/luskaner/ageServer/server/internal/routes/game/relationship"
	"github.com/luskaner/ageServer/server/internal/routes/matchcontent"
	"github.com/luskaner/ageServer/server/internal/routes/msstore"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type Game struct {
	Router
}

func (g *Game) Name() string {
	return "game"
}

func (g *Game) InitializeRoutes(gameId string, _ http.Handler) http.Handler {
	g.initialize()
	gameGroup := g.group.Subgroup("/game")
	itemGroup := gameGroup.Subgroup("/item")
	itemGroup.HandleFunc("GET", "/getItemDefinitionsJson", item.GetItemDefinitionsJson)
	itemGroup.HandleFunc("GET", "/getItemLoadouts", item.GetItemLoadouts)
	itemGroup.HandleFunc("POST", "/signItems", item.SignItems)
	if gameId != game.AoE1 {
		itemGroup.HandleFunc("GET", "/getItemBundleItemsJson", item.GetItemBundleItemsJson)
		itemGroup.HandleFunc("GET", "/getInventoryByProfileIDs", item.GetInventoryByProfileIDs)
		itemGroup.HandleFunc("POST", "/detachItems", item.DetachItems)
		itemGroup.HandleFunc("GET", "/getLevelRewardsTableJson", item.GetLevelRewardsTableJson)
		itemGroup.HandleFunc("POST", "/moveItem", item.MoveItem)
		itemGroup.HandleFunc("POST", "/updateItemAttributes", item.UpdateItemAttributes)
		itemGroup.HandleFunc("POST", "/createItemLoadout", item.CreateItemLoadout)
		itemGroup.HandleFunc("POST", "/equipItemLoadout", item.EquipItemLoadout)
		itemGroup.HandleFunc("POST", "/updateItemLoadout", item.UpdateItemLoadout)
	}
	itemGroup.HandleFunc("GET", "/getItemPrices", item.GetItemPrices)
	itemGroup.HandleFunc("GET", "/getScheduledSaleAndItems", item.GetScheduledSaleAndItems)
	itemGroup.HandleFunc("GET", "/getPersonalizedSaleItems", item.GetPersonalizedSaleItems)
	clanGroup := gameGroup.Subgroup("/clan")
	clanGroup.HandleFunc("POST", "/create", clan.Create)
	clanGroup.HandleFunc("GET", "/find", clan.Find)

	communityEventGroup := gameGroup.Subgroup("/CommunityEvent")
	communityEventGroup.HandleFunc("GET", "/getAvailableCommunityEvents", communityEvent.GetAvailableCommunityEvents)

	if gameId == game.AoE4 || gameId == game.AoM {
		communityEventGroup.HandleFunc("GET", "/getEventStats", communityEvent.GetEventStats)
		communityEventGroup.HandleFunc("GET", "/getEventLeaderboard", communityEvent.GetEventLeaderboard)
	}
	// TODO: GET getEventChallengeProgress ?
	// TODO: POST updateEventChallengeProgressBatched ?
	challengeGroup := gameGroup.Subgroup("/challenge")
	if gameId == game.AoE3 {
		challengeGroup.HandleFunc("POST", "/updateProgress", challenge.UpdateProgress)
	}
	if gameId == game.AoE4 || gameId == game.AoM {
		challengeGroup.HandleFunc("POST", "/updateProgressBatched", challenge.UpdateProgressBatched)
	}
	ChallengeGroup := gameGroup.Subgroup("/Challenge")
	if gameId == game.AoE3 {
		ChallengeGroup.HandleFunc("POST", "/getChallengeProgress", challenge.GetChallengeProgress)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		ChallengeGroup.HandleFunc("GET", "/getChallengeProgress", challenge.GetChallengeProgress)
	}
	if gameId == game.AoE4 {
		// FIXME: Implement as a separate route
		ChallengeGroup.HandleFunc("GET", "/getChallengeProgressByProfileID", challenge.GetChallengeProgress)
	}
	ChallengeGroup.HandleFunc("GET", "/getChallenges", challenge.GetChallenges)

	newsGroup := gameGroup.Subgroup("/news")
	newsGroup.HandleFunc("GET", "/getNews", news.GetNews)

	loginGroup := gameGroup.Subgroup("/login")
	loginHandler := login.Platformlogin
	if internal.Authentication != "disabled" {
		if internal.CanUseInternet {
			loginHandler = AuthMiddleware(loginHandler, gameId, internal.Authentication == "cached").ServeHTTP
		} else {
			loginHandler = AuthMiddlewareOffline(loginHandler).ServeHTTP
		}
	}
	loginHandler = LoginUserMiddleware(loginHandler).ServeHTTP
	loginGroup.HandleFunc("POST", "/platformlogin", loginHandler)
	loginGroup.HandleFunc("POST", "/logout", login.Logout)
	loginGroup.HandleFunc("POST", "/readSession", login.ReadSession)
	accountGroup := gameGroup.Subgroup("/account")
	accountGroup.HandleFunc("POST", "/setLanguage", account.SetLanguage)
	accountGroup.HandleFunc("POST", "/setCrossplayEnabled", account.SetCrossplayEnabled)
	accountGroup.HandleFunc("POST", "/setAvatarMetadata", account.SetAvatarMetadata)
	accountGroup.HandleFunc("POST", "/FindProfilesByPlatformID", account.FindProfilesByPlatformID)
	accountGroup.HandleFunc("GET", "/FindProfiles", account.FindProfiles)
	accountGroup.HandleFunc("GET", "/getProfileName", account.GetProfileName)
	if gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		accountGroup.HandleFunc("GET", "/getProfileProperty", account.GetProfileProperty)
		accountGroup.HandleFunc("POST", "/addProfileProperty", account.AddProfileProperty)
		accountGroup.HandleFunc("POST", "/clearProfileProperty", account.ClearProfileProperty)
	}

	LeaderboardGroup := gameGroup.Subgroup("/Leaderboard")
	if gameId == game.AoE3 {
		LeaderboardGroup.HandleFunc("POST", "/getRecentMatchHistory", leaderboard.GetRecentMatchHistory)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		LeaderboardGroup.HandleFunc("GET", "/getRecentMatchHistory", leaderboard.GetRecentMatchHistory)
	}
	LeaderboardGroup.HandleFunc("GET", "/getLeaderBoard", leaderboard.GetLeaderBoard)
	LeaderboardGroup.HandleFunc("GET", "/getAvailableLeaderboards", leaderboard.GetAvailableLeaderboards)
	LeaderboardGroup.HandleFunc("GET", "/getStatGroupsByProfileIDs", leaderboard.GetStatGroupsByProfileIDs)
	LeaderboardGroup.HandleFunc("GET", "/getStatsForLeaderboardByProfileName", leaderboard.GetStatsForLeaderboardByProfileName)
	LeaderboardGroup.HandleFunc("GET", "/getPartyStat", leaderboard.GetPartyStat)
	if gameId == game.AoE3 {
		LeaderboardGroup.HandleFunc("GET", "/getAvatarStatLeaderBoard", leaderboard.GetAvatarStatLeaderBoard)
	}
	if gameId == game.AoE4 {
		LeaderboardGroup.HandleFunc("GET", "/getRecentMatchSinglePlayerHistory", leaderboard.GetRecentMatchSinglePlayerHistory)
	}
	// Implement getPersonalStat ?
	leaderboardGroup := gameGroup.Subgroup("/leaderboard")
	leaderboardGroup.HandleFunc("POST", "/applyOfflineUpdates", leaderboard.ApplyOfflineUpdates)
	leaderboardGroup.HandleFunc("POST", "/setAvatarStatValues", leaderboard.SetAvatarStatValues)

	// Older versions
	if gameId == game.AoE4 {
		automatchGroup := gameGroup.Subgroup("/automatch")
		automatchGroup.HandleFunc("GET", "/getAutomatchMap", Automatch2.GetAutomatchMap)
	}
	automatch2Group := gameGroup.Subgroup("/automatch2")
	automatch2Group.HandleFunc("GET", "/getAutomatchMap", Automatch2.GetAutomatchMap)
	if gameId == game.AoE4 {
		automatch2Group.HandleFunc("POST", "/polling", Automatch2.Polling)
		automatch2Group.HandleFunc("POST", "/stoppolling", Automatch2.Stoppolling)
	} else if gameId == game.AoE2 {
		automatch2Group.HandleFunc("POST", "/polling", Automatch2.PollingAoE2)
		automatch2Group.HandleFunc("GET", "/polling", Automatch2.PollingAoE2)
		automatch2Group.HandleFunc("POST", "/stoppolling", Automatch2.StopPolling)
		automatch2Group.HandleFunc("GET", "/stoppolling", Automatch2.StopPolling)
		automatch2Group.HandleFunc("POST", "/updateStatus", Automatch2.UpdateStatus)
		automatch2Group.HandleFunc("GET", "/updateStatus", Automatch2.UpdateStatus)
	}
	AchievementGroup := gameGroup.Subgroup("/Achievement")
	AchievementGroup.HandleFunc("GET", "/getAchievements", achievement.GetAchievements)
	AchievementGroup.HandleFunc("GET", "/getAvailableAchievements", achievement.GetAvailableAchievements)

	achievementGroup := gameGroup.Subgroup("/achievement")
	achievementGroup.HandleFunc("POST", "/applyOfflineUpdates", achievement.ApplyOfflineUpdates)
	achievementGroup.HandleFunc("POST", "/grantAchievement", achievement.GrantAchievement)
	achievementGroup.HandleFunc("POST", "/syncStats", achievement.SyncStats)

	advertisementGroup := gameGroup.Subgroup("/advertisement")
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		advertisementGroup.HandleFunc("POST", "/updatePlatformSessionID", advertisement.UpdatePlatformSessionID)
	}
	advertisementGroup.HandleFunc("POST", "/join", advertisement.Join)
	// TODO: Review
	if gameId == game.AoE2 || gameId == game.AoE4 || gameId == game.AoM {
		advertisementGroup.HandleFunc("POST", "/updateTags", advertisement.UpdateTags)
	}
	advertisementGroup.HandleFunc("POST", "/update", advertisement.Update)
	advertisementGroup.HandleFunc("POST", "/leave", advertisement.Leave)
	advertisementGroup.HandleFunc("POST", "/host", advertisement.Host)
	if gameId == game.AoE1 || gameId == game.AoE3 {
		advertisementGroup.HandleFunc("POST", "/getLanAdvertisements", advertisement.GetLanAdvertisements)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 {
		advertisementGroup.HandleFunc("GET", "/getLanAdvertisements", advertisement.GetLanAdvertisements)
	}
	if gameId == game.AoE1 || gameId == game.AoE3 {
		advertisementGroup.HandleFunc("POST", "/updatePlatformLobbyID", advertisement.UpdatePlatformLobbyID)
	}
	if gameId == game.AoE3 {
		advertisementGroup.HandleFunc("POST", "/findObservableAdvertisements", advertisement.FindObservableAdvertisements)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		advertisementGroup.HandleFunc("GET", "/findObservableAdvertisements", advertisement.FindObservableAdvertisements)
	}
	advertisementGroup.HandleFunc("GET", "/getAdvertisements", advertisement.GetAdvertisements)
	if gameId == game.AoE1 || gameId == game.AoE3 {
		advertisementGroup.HandleFunc("POST", "/findAdvertisements", advertisement.FindAdvertisements)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		advertisementGroup.HandleFunc("GET", "/findAdvertisements", advertisement.FindAdvertisements)
	}
	// TODO: Check if AoE2/AoM use it too
	if gameId == game.AoE4 {
		advertisementGroup.HandleFunc("GET", "/getAdvertisementByPlatformSessionID", advertisement.GetAdvertisementByPlatformSessionId)
	}

	advertisementGroup.HandleFunc("POST", "/updateState", advertisement.UpdateState)

	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		advertisementGroup.HandleFunc("POST", "/startObserving", advertisement.StartObserving)
		advertisementGroup.HandleFunc("POST", "/stopObserving", advertisement.StopObserving)
	}

	chatGroup := gameGroup.Subgroup("/chat")
	if gameId == game.AoE1 || gameId == game.AoE3 {
		chatGroup.HandleFunc("POST", "/getChatChannels", chat.GetChatChannels)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		chatGroup.HandleFunc("GET", "/getChatChannels", chat.GetChatChannels)
	}
	chatGroup.HandleFunc("GET", "/getOfflineMessages", chat.GetOfflineMessages)
	if gameId == game.AoE3 {
		chatGroup.HandleFunc("POST", "/joinChannel", chat.JoinChannel)
	}
	if gameId == game.AoE3 {
		chatGroup.HandleFunc("POST", "/leaveChannel", chat.LeaveChannel)
	}
	if gameId == game.AoE3 {
		chatGroup.HandleFunc("POST", "/sendText", chat.SendText)
	}
	if gameId == game.AoE3 {
		chatGroup.HandleFunc("POST", "/sendWhisper", func(writer http.ResponseWriter, request *http.Request) {
			chat.SendWhisper(writer, request, true)
		})
	}
	if gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		chatGroup.HandleFunc("POST", "/sendWhispers", func(writer http.ResponseWriter, request *http.Request) {
			chat.SendWhisper(writer, request, false)
		})
	}
	if gameId == game.AoM {
		chatGroup.HandleFunc("POST", "/deleteOfflineMessage", chat.DeleteOfflineMessage)
	}

	relationshipGroup := gameGroup.Subgroup("/relationship")
	if gameId == game.AoE1 || gameId == game.AoE3 {
		relationshipGroup.HandleFunc("POST", "/getRelationships", relationship.GetRelationships)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		relationshipGroup.HandleFunc("GET", "/getRelationships", relationship.GetRelationships)
	}
	relationshipGroup.HandleFunc("GET", "/getPresenceData", relationship.GetPresenceData)
	relationshipGroup.HandleFunc("POST", "/setPresence", relationship.SetPresence)
	if gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		relationshipGroup.HandleFunc("POST", "/setPresenceProperty", relationship.SetPresenceProperty)
	}
	if gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		relationshipGroup.HandleFunc("POST", "/addfriend", relationship.Addfriend)
	}
	relationshipGroup.HandleFunc("POST", "/ignore", relationship.Ignore)
	relationshipGroup.HandleFunc("POST", "/clearRelationship", relationship.ClearRelationship)

	partyGroup := gameGroup.Subgroup("/party")
	partyGroup.HandleFunc("POST", "/peerAdd", party.PeerAdd)
	partyGroup.HandleFunc("POST", "/peerUpdate", party.PeerUpdate)
	partyGroup.HandleFunc("POST", "/sendMatchChat", party.SendMatchChat)
	partyGroup.HandleFunc("POST", "/reportMatch", party.ReportMatch)
	partyGroup.HandleFunc("POST", "/finalizeReplayUpload", party.FinalizeReplayUpload)
	partyGroup.HandleFunc("POST", "/updateHost", party.UpdateHost)
	// TODO: Review
	if gameId == game.AoE4 || gameId == game.AoM {
		partyGroup.HandleFunc("POST", "/createOrReportSinglePlayer", party.CreateOrReportSinglePlayer)
	}
	playerReportGroup := gameGroup.Subgroup("/playerreport")
	// TODO: Check if it applies to AoE III
	if gameId == game.AoE2 || gameId == game.AoE4 || gameId == game.AoM {
		playerReportGroup.HandleFunc("POST", "/reportUser", playerreport.ReportUser)
	}

	invitationGroup := gameGroup.Subgroup("/invitation")
	invitationGroup.HandleFunc("POST", "/extendInvitation", invitation.ExtendInvitation)
	invitationGroup.HandleFunc("POST", "/cancelInvitation", invitation.CancelInvitation)
	invitationGroup.HandleFunc("POST", "/replyToInvitation", invitation.ReplyToInvitation)

	cloudGroup := gameGroup.Subgroup("/cloud")
	if gameId == game.AoE3 {
		cloudGroup.HandleFunc("POST", "/getFileURL", cloud.GetFileURL)
	}
	if gameId == game.AoE2 || gameId == game.AoE3 || gameId == game.AoE4 {
		cloudGroup.HandleFunc("GET", "/getFileURL", cloud.GetFileURL)
	}

	cloudGroup.HandleFunc("GET", "/getTempCredentials", cloud.GetTempCredentials)

	msstoreGroup := gameGroup.Subgroup("/msstore")
	msstoreGroup.HandleFunc("GET", "/getStoreTokens", msstore.GetStoreTokens)

	g.group.HandleFunc("GET", "/wss/", wss.Handle)

	// Artificial path needed for cloudGroup
	// TODO: Enable to AoM if/when it gets cloud support
	if gameId == game.AoE2 || gameId == game.AoE3 {
		g.group.HandleFunc("GET", "/cloudfiles/", cloudfiles.Cloudfiles)
	}

	// Custom files (savegames/replays). Registered here and in
	// apiAgeOfEmpires.go because each title reaches them through a different
	// host, so no gameId condition is needed.
	g.group.HandleFunc("POST", "/api/MatchContent/", matchcontent.Upload)
	g.group.HandleFunc("GET", "/api/MatchContent/{matchId}", matchcontent.Download)
	g.group.HandleFunc("GET", "/api/MatchContent/{matchId}/file/{hash}", matchcontent.DownloadFile)
	return SessionMiddleware(g.group.mux)
}
