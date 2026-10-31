package models

import (
	"net/http"

	"github.com/luskaner/ageServer/server/internal"
)

type Game interface {
	Title() string
	Resources() Resources
	Items() Items
	LeaderboardDefinitions() LeaderboardDefinitions
	PresenceDefinitions() PresenceDefinitions
	BattleServers() BattleServers
	Users() Users
	Advertisements() Advertisements
	ChatChannels() ChatChannels
	Sessions() Sessions
	CustomFiles() CustomFiles
	Matchmaking() Matchmaking
	MatchHistory() MatchHistory
}

type MatchHistory interface {
	RecordMatch(MatchResult)
	UpdateRatings(MatchResult)
	HasMatch(matchID int64) bool
	GetRatings(profileIDs []int32) map[int32]*PlayerRating
	GetRatingsForLeaderboard(leaderboardID int32, profileIDs []int32) map[int32]*PlayerRating
	GetRecentForProfile(profileID int32, limit int) []MatchResult
	GetRecentForProfiles(profileIDs []int32, limit int) []MatchResult
	GetLeaderboard(limit int) []*PlayerRating
	GetLeaderboardPage(leaderboardID int32, start, count int) ([]*PlayerRating, int64)
	EnsureRating(profileID int32, statID int32) *PlayerRating
	GetDefaultRating() int32
	// UpsertProfile stores the identity of a profile (alias, platform user id,
	// stat group id...) so the ladder can render players that are offline.
	UpsertProfile(p StoredProfile)
	// GetProfiles returns the stored identity of the requested profiles.
	GetProfiles(profileIDs []int32) map[int32]*StoredProfile
	// RecordPendingMatch stores the pairing (map, queue, players) as soon as the
	// match is created, so the result can still be attributed after the
	// matchmaking entry has been removed.
	RecordPendingMatch(m MatchResult)
	// ApplyResult stores a reported result and returns the updated ratings and
	// whether this call moved them.
	ApplyResult(m MatchResult) (map[int32]*PlayerRating, bool)
}

func G(r *http.Request) Game {
	return Gg[Game](r)
}

func Gg[T Game](r *http.Request) T {
	return r.Context().Value("game").(T)
}

type MainGame struct {
	battleServers          BattleServers
	resources              Resources
	users                  Users
	advertisements         Advertisements
	chatChannels           ChatChannels
	sessions               Sessions
	matchmaking            Matchmaking
	leaderboardDefinitions LeaderboardDefinitions
	items                  Items
	presenceDefinitions    PresenceDefinitions
	customFiles            CustomFiles
	matchHistory           MatchHistory
	title                  string
}

type CreateMainGameOpts struct {
	Resources    *ResourcesOpts
	BattleServer *BattleServerOpts
	Instances    *InstanceOpts
}

type InstanceOpts struct {
	Resources              Resources
	BattleServers          BattleServers
	Users                  Users
	Advertisements         Advertisements
	ChatChannels           ChatChannels
	Sessions               Sessions
	Matchmaking            Matchmaking
	LeaderboardDefinitions LeaderboardDefinitions
	PresenceDefinitions    PresenceDefinitions
	Items                  Items
	CustomFiles            CustomFiles
	CustomFilesOpts        *CustomFilesOpts
}

func CreateMainGame(gameId string, opts *CreateMainGameOpts) Game {
	if opts == nil {
		opts = &CreateMainGameOpts{}
	}
	if opts.Resources == nil {
		opts.Resources = &ResourcesOpts{}
	}
	if opts.Instances == nil {
		opts.Instances = &InstanceOpts{}
	}
	if opts.Instances.BattleServers == nil {
		opts.Instances.BattleServers = &MainBattleServers{}
	}
	if opts.Instances.Resources == nil {
		opts.Instances.Resources = &MainResources{}
	}
	if opts.Instances.Users == nil {
		opts.Instances.Users = &MainUsers{}
	}
	if opts.Instances.Advertisements == nil {
		opts.Instances.Advertisements = &MainAdvertisements{}
	}
	if opts.Instances.ChatChannels == nil {
		opts.Instances.ChatChannels = &MainChatChannels{}
	}
	if opts.Instances.Sessions == nil {
		opts.Instances.Sessions = &MainSessions{}
	}
	if opts.Instances.CustomFiles == nil {
		opts.Instances.CustomFiles = &MainCustomFiles{}
	}
	if opts.Instances.Matchmaking == nil {
		opts.Instances.Matchmaking = &MainMatchmaking{}
	}
	game := &MainGame{
		battleServers:  opts.Instances.BattleServers,
		resources:      opts.Instances.Resources,
		users:          opts.Instances.Users,
		advertisements: opts.Instances.Advertisements,
		chatChannels:   opts.Instances.ChatChannels,
		sessions:       opts.Instances.Sessions,
		customFiles:    opts.Instances.CustomFiles,
		matchmaking:    opts.Instances.Matchmaking,
		title:          gameId,
	}
	game.battleServers.Initialize(BattleServersStore[gameId], opts.BattleServer)
	game.resources.Initialize(gameId, opts.Resources)
	// Extract challenges.json from resources for games with challenge progress
	if mainUsers, ok := game.users.(*MainUsers); ok && mainUsers != nil {
		if challengesData, ok := game.resources.SignedAssets()["challenges.json"]; ok {
			mainUsers.challengesData = challengesData
			// Tell the upgrader which JSON format this game uses
			mainUsers.challengesFormat = getChallengeFormat(gameId)
		}
	}
	game.users.Initialize()
	game.advertisements.Initialize(game.users, game.battleServers)
	game.chatChannels.Initialize(game.resources.ChatChannels())
	game.sessions.Initialize()
	game.customFiles.Initialize(gameId, opts.Instances.CustomFilesOpts)
	game.matchHistory = NewMatchHistoryStoreForGame(gameId)
	if mainUsers, ok := game.users.(*MainUsers); ok && mainUsers != nil {
		history := game.matchHistory
		mainUsers.SetRatingSource(func(profileID int32) int32 {
			if rating, found := history.GetRatings([]int32{profileID})[profileID]; found && rating != nil {
				return rating.Rating
			}
			return 0
		})
		mainUsers.SetProfileSink(func(u User) {
			profile := StoredProfile{
				ProfileID:      u.GetId(),
				StatGroupID:    u.GetStatGroupID(),
				Alias:          u.GetAlias(),
				PlatformUserID: u.GetPlatformUserID(),
				IsXbox:         u.GetXbox(),
				XP:             u.GetProfileExperience(),
				Level:          u.GetProfileLevel(),
				Region:         u.GetPlatformRelated(),
			}
			if metadata := u.GetAvatarMetadata(); metadata != nil {
				_ = metadata.WithReadOnly(func(value *string) error {
					if value != nil {
						profile.AvatarMetadata = *value
					}
					return nil
				})
			}
			history.UpsertProfile(profile)
		})
	}
	// Backfill profiles from existing user data files
	if game.matchHistory != nil {
		userDataDir := ""
		if game.resources != nil {
			if rd, ok := interface{}(game.resources).(interface{ ResourcesFolder() string }); ok {
				if r := rd.ResourcesFolder(); r != "" {
					userDataDir = r + "\\userData\\" + gameId
				}
			}
		}
		if store, ok := game.matchHistory.(*MatchHistoryStore); ok {
			store.BackfillProfiles(userDataDir)
		}
	}
	game.matchmaking.Initialize(game)
	if itemLocations, ok := game.resources.ArrayFiles()["itemLocations.json"]; ok {
		if opts.Instances.Items == nil {
			opts.Instances.Items = &MainItems{}
		}
		game.items = opts.Instances.Items
		itemDefinitions, _ := game.resources.SignedAssets()["itemDefinitions.json"]
		game.items.Initialize(itemDefinitions, itemLocations)
	}
	if leaderboards, ok := game.resources.ArrayFiles()["leaderboards.json"]; ok {
		if opts.Instances.LeaderboardDefinitions == nil {
			opts.Instances.LeaderboardDefinitions = &MainLeaderboardDefinitions{}
		}
		game.leaderboardDefinitions = opts.Instances.LeaderboardDefinitions
		game.leaderboardDefinitions.Initialize(leaderboards)
	}
	if opts.Instances.PresenceDefinitions == nil {
		opts.Instances.PresenceDefinitions = &MainPresenceDefinitions{}
	}
	var presenceDefinitions internal.A
	if loadedPresenceDefinitions, ok := game.resources.ArrayFiles()["presenceData.json"]; ok {
		presenceDefinitions = loadedPresenceDefinitions
	}
	game.presenceDefinitions = opts.Instances.PresenceDefinitions
	game.presenceDefinitions.Initialize(presenceDefinitions)
	return game
}

func (g *MainGame) Resources() Resources {
	return g.resources
}

func (g *MainGame) Users() Users {
	return g.users
}

func (g *MainGame) Advertisements() Advertisements {
	return g.advertisements
}

func (g *MainGame) ChatChannels() ChatChannels {
	return g.chatChannels
}

func (g *MainGame) Title() string {
	return g.title
}

func (g *MainGame) BattleServers() BattleServers {
	return g.battleServers
}

func (g *MainGame) Sessions() Sessions {
	return g.sessions
}

func (g *MainGame) Matchmaking() Matchmaking {
	return g.matchmaking
}

func (g *MainGame) MatchHistory() MatchHistory {
	return g.matchHistory
}

func (g *MainGame) LeaderboardDefinitions() LeaderboardDefinitions {
	return g.leaderboardDefinitions
}

func (g *MainGame) Items() Items {
	return g.items
}

func (g *MainGame) PresenceDefinitions() PresenceDefinitions {
	return g.presenceDefinitions
}

func (g *MainGame) CustomFiles() CustomFiles {
	return g.customFiles
}

// getChallengeFormat returns the JSON format used by a game's challenges.json.
func getChallengeFormat(gameId string) ChallengeFormat {
	switch gameId {
	case "age4":
		return FormatAoE4
	case "age2", "athens": // AoE II and AoM (Athens) use flat array format
		return FormatFlatArray
	default:
		return FormatAoE4 // default to AoE4 format
	}
}
