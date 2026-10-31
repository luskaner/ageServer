package models

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

type User interface {
	GetId() int32
	GetXbox() bool
	GetStatGroupID() int32
	GetProfileId() int32
	GetReliclink() int32
	GetAlias() string
	GetPlatformPath() string
	GetAccountType() int
	GetPlatformUserID() uint64
	EncodeExtraProfileInfo(clientLibVersion uint16) i.A
	EncodeLeaderboardStat(r *PlayerRating) i.A
	EncodeProfileInfo(clientLibVersion uint16) i.A
	GetPresence() int32
	SetPresence(presence int32)
	SetPresenceProperty(id int32, value string)
	GetAvatarMetadata() *PersistentJsonData[*string]
	GetProfileProperties() *PersistentJsonData[*map[string]string]
	GetProfileExperience() uint32
	GetProfileLevel() uint16
	GetPlatformRelated() uint8
	GetAvatarStats() *PersistentJsonData[*AvatarStats]
	GetPersistentData() *PersistentStringJsonMap
	GetItems() *PersistentJsonData[*map[int32]*MainItem]
	GetItemLoadouts() *PersistentJsonData[*MainItemLoadouts]
	GetChallengeProgress() *PersistentJsonData[*[]ChallengeProgress]
	GetAuth() *PersistentJsonData[*time.Time]
	EncodeStats() i.A
	EncodePresence(definitions PresenceDefinitions) i.A
	// GetRating/SetRating expose the cached 1v1 random map rating that is
	// embedded in the extra profile info sent to the game client.
	GetRating() int32
	SetRating(rating int32)
}

type MainUser struct {
	id                int32
	statGroupID       int32
	alias             string
	platformUserId    uint64
	profileId         int32
	profileExperience uint32
	reliclink         int32
	isXbox            bool
	persistentData    *PersistentStringJsonMap
	// Dynamic from here
	profileProperties  *PersistentJsonData[*map[string]string]
	avatarMetadata     *PersistentJsonData[*string]
	presence           atomic.Int32
	presenceProperties *i.SafeMap[int32, string]
	avatarStats        *PersistentJsonData[*AvatarStats]
	items              *PersistentJsonData[*map[int32]*MainItem]
	itemLoadouts       *PersistentJsonData[*MainItemLoadouts]
	challengeProgress  *PersistentJsonData[*[]ChallengeProgress]
	auth               *PersistentJsonData[*time.Time]
	rating             atomic.Int32
}

// EncodeStats encodes the value for the client's stats field.
func (u *MainUser) EncodeStats() i.A {
	var result i.A
	_ = u.GetAvatarStats().WithReadOnly(func(data *AvatarStats) error {
		result = data.Encode(u.GetProfileId())
		return nil
	})
	return result
}

type Users interface {
	Initialize()
	GetOrCreateUser(gameId string, itemDefinitions Items, avatarStatsDefinitions AvatarStatDefinitions, remoteAddr string, remoteMacAddress string, isXbox bool, platformUserId uint64, alias string) User
	GetUserByStatGroupID(statGroupID int32) (User, bool)
	GetUserById(id int32) (User, bool)
	GetUserIds() func(func(int32) bool)
	EncodeProfileInfo(definitions PresenceDefinitions, matches func(user User) bool, clientLibVersion uint16) []i.A
	GetUserByPlatformUserId(xbox bool, id uint64) (User, bool)
	// FlushAll calls Flush on every user's persistent data. It is called at
	// server shutdown to give pending writes a final chance to reach disk.
	FlushAll()
}

type MainUsers struct {
	store      *i.SafeMap[string, User]
	GenerateFn func(
		gameId string,
		persistentData *PersistentStringJsonMap,
		itemDefinitions Items,
		avatarStatsDefinitions AvatarStatDefinitions,
		identifier string,
		isXbox bool,
		platformUserId uint64,
		alias string,
	) User
	// StoreFactory chooses where each user's data is kept. Replacing it is the
	// only change needed to run the server on a real database instead of the
	// default per-key JSON files.
	StoreFactory UserStoreFactory
	// ratingSource, when set, loads the persisted 1v1 rating of a profile. It
	// is called once per account load so the rating survives a server restart
	// even before the client asks for the ladder.
	ratingSource func(profileID int32) int32
	// profileSink, when set, persists the identity of every user the server
	// builds, so the ladder can still render players that are offline.
	profileSink func(User)
	// challengesData holds the raw challenges.json data for initializing
	// challenge progress. It is populated by CreateMainGame from the Resources.
	challengesData []byte
	// challengesFormat tells the upgrader how to parse challenges.json
	challengesFormat ChallengeFormat
}

// SetRatingSource installs the callback used to hydrate ratings on account
// load. It is wired by the game once the match history store exists.
func (users *MainUsers) SetRatingSource(source func(profileID int32) int32) {
	users.ratingSource = source
}

// SetProfileSink installs the callback used to remember the identity of every
// user the server builds. It is wired by the game once the match history store
// exists, and it is what keeps the ladder usable for offline players.
func (users *MainUsers) SetProfileSink(sink func(User)) {
	users.profileSink = sink
}

func (users *MainUsers) Initialize() {
	users.store = i.NewSafeMap[string, User]()
	if users.GenerateFn == nil {
		users.GenerateFn = users.Generate
	}
	if users.StoreFactory == nil {
		users.StoreFactory = JSONUserStoreFactory
	}
}

func (users *MainUsers) Generate(gameId string, persistentData *PersistentStringJsonMap, itemDefinitions Items, avatarStatsDefinitions AvatarStatDefinitions, identifier string, isXbox bool, platformUserId uint64, alias string) User {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(identifier))
	hsh := hasher.Sum(nil)
	seed := binary.BigEndian.Uint64(hsh)
	rng := rand.New(rand.NewPCG(seed, seed))
	var avatarStats *PersistentJsonData[*AvatarStats]
	if avatarStatsDefinitions != nil {
		avatarStats = OpenPersistentJsonData[*AvatarStats](
			persistentData,
			identifier,
			"avatarStats",
			NewAvatarStatsUpgradableDefaultData(gameId, avatarStatsDefinitions),
		)
	}
	var profileProperties *PersistentJsonData[*map[string]string]
	if gameId == game.AoE3 || gameId == game.AoE4 || gameId == game.AoM {
		profileProperties = OpenPersistentJsonData[*map[string]string](
			persistentData,
			identifier,
			"profileProperties",
			NewProfilePropertiesUpgradableDefaultData(),
		)
	}
	var items *PersistentJsonData[*map[int32]*MainItem]
	if itemDefinitions != nil {
		items = OpenPersistentJsonData[*map[int32]*MainItem](
			persistentData,
			identifier,
			"items",
			NewItemsUpgradableDefaultData(gameId, itemDefinitions),
		)
	}
	var itemLoadouts *PersistentJsonData[*MainItemLoadouts]
	if gameId != game.AoE1 {
		itemLoadouts = OpenPersistentJsonData[*MainItemLoadouts](
			persistentData,
			identifier,
			"itemLoadouts",
			NewItemLoadoutsUpgradableDefaultData(),
		)
	}
	var challengeProgress *PersistentJsonData[*[]ChallengeProgress]
	if users.challengesData != nil {
		challengeProgress = OpenPersistentJsonData[*[]ChallengeProgress](
			persistentData,
			identifier,
			"challenges",
			NewChallengeProgressUpgradableDefaultData(users.challengesData, users.challengesFormat),
		)
		if challengeProgress != nil {
			// Reconcile the saved progress with the challenges of the current
			// resources on every user load: definitions added by a resource
			// update are included and IDs stay sequential, while the progress
			// the client already reported is kept.
			upgrader := NewChallengeProgressUpgradableDefaultData(users.challengesData, users.challengesFormat)
			if err := challengeProgress.WithReadWrite(func(data *[]ChallengeProgress) error {
				*data = reconcileChallengeProgress(*data, upgrader)
				return nil
			}); err != nil {
				logger.Fail("User %s: challenge progress could not be reconciled (%s)", identifier, err)
			}
		}
	}
	avatarMetadata := OpenPersistentJsonData[*string](
		persistentData,
		identifier,
		"avatarMetadata",
		NewAvatarMetadataUpgradableDefaultData(gameId),
	)
	auth := OpenPersistentJsonData[*time.Time](
		persistentData,
		identifier,
		"auth",
		NewAuthUpgradableDefaultData(),
	)
	var presenceProperties *i.SafeMap[int32, string]
	if gameId != game.AoE1 {
		presenceProperties = i.NewSafeMap[int32, string]()
	}
	return &MainUser{
		id:                 rng.Int32(),
		statGroupID:        rng.Int32(),
		profileId:          rng.Int32(),
		avatarMetadata:     avatarMetadata,
		items:              items,
		itemLoadouts:       itemLoadouts,
		challengeProgress:  challengeProgress,
		profileProperties:  profileProperties,
		reliclink:          rng.Int32(),
		alias:              alias,
		platformUserId:     platformUserId,
		isXbox:             isXbox,
		avatarStats:        avatarStats,
		persistentData:     persistentData,
		presenceProperties: presenceProperties,
		auth:               auth,
	}
}

func generatePlatformUserIdSteam(rng *rand.Rand) uint64 {
	Z := rng.Int64N(1 << 31)
	Y := Z % 2
	id := Z*2 + Y + 76561197960265728
	return uint64(id)
}

func generateFullPlatformUserIdXbox(platformUserId int64) string {
	rng := rand.New(rand.NewPCG(uint64(platformUserId), uint64(platformUserId)))
	const chars = "0123456789ABCDEF"
	id := make([]byte, 40)
	for j := range id {
		id[j] = chars[rng.N(len(chars))]
	}
	return string(id)
}

func generatePlatformUserIdXbox(rng *rand.Rand) uint64 {
	return uint64(rng.Int64N(9e15) + 1e15)
}

func (users *MainUsers) GetOrCreateUser(gameId string, itemDefinitions Items, avatarStatsDefinitions AvatarStatDefinitions, remoteAddr string, remoteMacAddress string, isXbox bool, platformUserId uint64, alias string) User {
	if i.GeneratePlatformUserId {
		entropy := make([]byte, 16)
		macAddress, err := net.ParseMAC(remoteMacAddress)
		if err == nil {
			copy(entropy[:6], macAddress)
		}
		ipStr, _, err := net.SplitHostPort(remoteAddr)
		if err == nil {
			ip := net.ParseIP(ipStr)
			if ip != nil {
				ipV4 := ip.To4()
				if ipV4 != nil {
					copy(entropy[6:10], ipV4)
				}
			}
		}
		sizeAlias := min(len(alias), 6)
		copy(entropy[10:10+sizeAlias], alias[:sizeAlias])
		seed1 := binary.BigEndian.Uint64(entropy[:8])
		seed2 := binary.BigEndian.Uint64(entropy[8:])
		rng := rand.New(rand.NewPCG(seed1, seed2))
		if isXbox {
			platformUserId = generatePlatformUserIdXbox(rng)
		} else {
			platformUserId = generatePlatformUserIdSteam(rng)
		}
	}
	identifier := getPlatformPath(isXbox, platformUserId)
	mainUser, _ := users.store.LoadOrStoreFn(
		identifier,
		func() User {
			userStore, err := users.StoreFactory(gameId, isXbox, platformUserId)
			if err != nil {
				logger.Fail("Could not open the save data of %s: %s", identifier, err)
				return nil
			}
			persistentData, err := NewPersistentStringMap(userStore)
			if err != nil {
				logger.Fail("Could not read the save data of %s: %s", identifier, err)
				return nil
			}
			return users.GenerateFn(
				gameId,
				persistentData,
				itemDefinitions,
				avatarStatsDefinitions,
				identifier,
				isXbox,
				platformUserId,
				alias,
			)
		},
	)
	if mainUser == nil {
		// Generation failed; remove the cached entry so the next attempt
		// can retry instead of permanently serving a nil user.
		users.store.Delete(identifier)
	} else {
		if users.ratingSource != nil {
			if rating := users.ratingSource(mainUser.GetId()); rating > 0 {
				mainUser.SetRating(rating)
			}
		}
		if users.profileSink != nil {
			users.profileSink(mainUser)
		}
	}
	return mainUser
}

func (users *MainUsers) GetUserByStatGroupID(statGroupID int32) (User, bool) {
	return users.getFirst(func(u User) bool { return u.GetStatGroupID() == statGroupID })
}

func (users *MainUsers) GetUserById(id int32) (User, bool) {
	return users.getFirst(func(u User) bool { return u.GetId() == id })
}

func (users *MainUsers) GetUserByPlatformUserId(xbox bool, id uint64) (User, bool) {
	return users.getFirst(func(u User) bool { return u.GetXbox() == xbox && u.GetPlatformUserID() == id })
}

func (users *MainUsers) getFirst(fn func(u User) bool) (User, bool) {
	for u := range users.store.Values() {
		if fn(u) {
			return u, true
		}
	}
	return nil, false
}

func (users *MainUsers) GetUserIds() func(func(int32) bool) {
	return func(yield func(int32) bool) {
		for u := range users.store.Values() {
			if !yield(u.GetId()) {
				return
			}
		}
	}
}

func (users *MainUsers) FlushAll() {
	for u := range users.store.Values() {
		if mu, ok := u.(*MainUser); ok {
			if pd := mu.GetPersistentData(); pd != nil {
				if err := pd.Flush(); err != nil {
					logger.Fail("User %s: flush failed: %s", mu.GetPlatformPath(), err)
				}
			}
		}
	}
}

func (users *MainUsers) EncodeProfileInfo(presenceDefinitions PresenceDefinitions, matches func(user User) bool, clientLibVersion uint16) []i.A {
	var profileInfo = make([]i.A, 0)
	for u := range users.store.Values() {
		if matches(u) {
			currentProfileInfo := u.EncodeProfileInfo(clientLibVersion)
			if presenceDefinitions != nil {
				currentProfileInfo = append(currentProfileInfo, u.EncodePresence(presenceDefinitions)...)
			}
			profileInfo = append(profileInfo, currentProfileInfo)
		}
	}
	return profileInfo
}

func (u *MainUser) GetPersistentData() *PersistentStringJsonMap {
	return u.persistentData
}

func (u *MainUser) GetAvatarStats() *PersistentJsonData[*AvatarStats] {
	return u.avatarStats
}

func (u *MainUser) GetId() int32 {
	return u.id
}

func (u *MainUser) GetXbox() bool {
	return u.isXbox
}

func (u *MainUser) GetStatGroupID() int32 {
	return u.statGroupID
}

func (u *MainUser) GetProfileId() int32 {
	return u.profileId
}

func (u *MainUser) GetProfileProperties() *PersistentJsonData[*map[string]string] {
	return u.profileProperties
}

func (u *MainUser) GetReliclink() int32 {
	return u.reliclink
}

func (u *MainUser) GetAlias() string {
	return u.alias
}

func getPlatformPath(isXbox bool, platformUserId uint64) string {
	var prefix string
	var fullPlatformUserId string
	if isXbox {
		fullPlatformUserId = generateFullPlatformUserIdXbox(int64(platformUserId))
		prefix = "xboxlive"
	} else {
		fullPlatformUserId = strconv.FormatUint(platformUserId, 10)
		prefix = "steam"
	}
	return fmt.Sprintf("/%s/%s", prefix, fullPlatformUserId)
}

func (u *MainUser) GetPlatformPath() string {
	return getPlatformPath(u.isXbox, u.platformUserId)
}

func (u *MainUser) GetAccountType() int {
	var prefix int
	if u.isXbox {
		prefix = 9
	} else {
		prefix = 3
	}
	return prefix
}

func (u *MainUser) GetPlatformUserID() uint64 {
	return u.platformUserId
}

// EncodeExtraProfileInfo returns the stat summary embedded in the account and
// profile payloads. It is a single leaderboardStat-shaped array (same order as
// EncodeLeaderboardStat) for the default ladder of the profile.
func (u *MainUser) EncodeExtraProfileInfo(clientLibVersion uint16) i.A {
	info := i.A{
		u.GetStatGroupID(), // statGroupID
		0,                  // leaderboardID
		0,                  // wins
		0,                  // losses
		-1,                 // streak
		0,                  // disputes
		0,                  // drops
		-1,                 // ranking
		-1,                 // rankTotal
		-1,                 // regionRanking
		-1,                 // regionRankTotal
		-1,                 // level
		u.GetRating(),      // rating
		// Some time in the past
		1713372625, // lastMatchTime
		0,          // highestRank
		0,          // highestRankLevel
		0,          // highestRating
	}
	if clientLibVersion >= i.V190 {
		info = append(info, 0, 0) // platformRank, platformRankTotal
	}
	return info
}

// EncodeLeaderboardStat returns one element of the leaderboardStats array that
// the Leaderboard endpoints (getLeaderBoard, getStatsForLeaderboardByProfileName,
// getStatGroupsByProfileIDs, getPartyStat) return. The client deserialises it
// positionally, so the order below must not change:
//
//	[statGroupID, leaderboardID, wins, losses, streak, disputes, drops,
//	 ranking, rankTotal, regionRanking, regionRankTotal, level, rating,
//	 lastMatchTime, highestRank, highestRankLevel, highestRating,
//	 platformRank, platformRankTotal]
func (u *MainUser) EncodeLeaderboardStat(r *PlayerRating) i.A {
	return EncodeLeaderboardStatFor(u.GetStatGroupID(), u.GetRating(), r)
}

// EncodeLeaderboardStatFor builds the same leaderboardStat for a profile that
// is not loaded in memory. statGroupID and fallbackRating come from the stored
// profile.
func EncodeLeaderboardStatFor(statGroupID int32, fallbackRating int32, r *PlayerRating) i.A {
	leaderboardID := int32(SoloRMRanked)
	rating := fallbackRating
	if rating <= 0 {
		rating = DefaultRating
	}
	highestRating := rating
	var wins, losses, streak int32
	ranking := int64(-1)
	rankTotal := int64(-1)
	lastMatch := int64(0)
	if r != nil {
		leaderboardID = r.LeaderboardID
		rating = r.Rating
		if r.HighestRating > 0 {
			highestRating = r.HighestRating
		}
		wins = r.Wins
		losses = r.Losses
		streak = r.Streak
		if r.Rank > 0 {
			ranking = r.Rank
		}
		if r.RankTotal > 0 {
			rankTotal = r.RankTotal
		}
		if !r.LastMatch.IsZero() {
			lastMatch = r.LastMatch.Unix()
		}
	}
	return i.A{
		statGroupID,   // statGroupID
		leaderboardID, // leaderboardID
		wins,          // wins
		losses,        // losses
		streak,        // streak
		0,             // disputes
		0,             // drops
		ranking,       // ranking
		rankTotal,     // rankTotal
		-1,            // regionRanking
		-1,            // regionRankTotal
		0,             // level
		rating,        // rating
		lastMatch,     // lastMatchTime
		ranking,       // highestRank
		0,             // highestRankLevel
		highestRating, // highestRating
		-1,            // platformRank
		-1,            // platformRankTotal
	}
}

// GetRating returns the cached 1v1 random map rating. Profiles that have never
// been rated get DefaultRating.
func (u *MainUser) GetRating() int32 {
	if rating := u.rating.Load(); rating > 0 {
		return rating
	}
	return DefaultRating
}

// SetRating caches the rating that will be embedded in the next
// EncodeExtraProfileInfo call.
func (u *MainUser) SetRating(rating int32) {
	if rating > 0 {
		u.rating.Store(rating)
	}
}

// profileEntityVersion is the fixed "last modified" timestamp sent with every
// profile. The client only needs it to be stable, so a constant is enough.
var profileEntityVersion = time.Date(2024, 5, 2, 3, 34, 0, 0, time.UTC).Unix()

func (u *MainUser) EncodeProfileInfo(clientLibVersion uint16) i.A {
	platformUserID := strconv.FormatUint(u.GetPlatformUserID(), 10)
	profileInfo := i.A{
		profileEntityVersion,
		u.GetId(),
		u.GetPlatformPath(),   // name
		u.GetAvatarMetadata(), // metaData
		u.GetAlias(),
	}
	if clientLibVersion >= i.V190 {
		profileInfo = append(profileInfo, u.GetAlias()) // sanitizedAlias
	}
	profileInfo = append(
		profileInfo,
		"",                       // clanName: empty = no clan
		u.GetStatGroupID(),       // personalStatGroupID
		u.GetProfileExperience(), // xp
		u.GetProfileLevel(),
		u.GetPlatformRelated(), // leaderboardRegionID
		nil,                    // banInfo: not banned
		platformUserID,
		u.GetAccountType(), // 3 = Steam, 9 = Xbox
		i.A{},              // linkedPlatformAccounts: empty = none
	)
	return profileInfo
}

// PlaceholderProfile is the identity used for a rating whose profile was never
// stored. It keeps the entry visible in the ladder without borrowing another
// player's alias; the real identity replaces it when the player connects.
func PlaceholderProfile(profileID int32, statGroupID int32) *StoredProfile {
	if statGroupID == 0 {
		statGroupID = profileID
	}
	return &StoredProfile{
		ProfileID:      profileID,
		StatGroupID:    statGroupID,
		Alias:          "Player " + strconv.FormatInt(int64(profileID), 10),
		PlatformUserID: uint64(profileID),
		Level:          1,
	}
}

// EncodeStoredProfileInfo builds the same profileInfo for a player that is not
// connected, using the identity stored in the match history database.
func EncodeStoredProfileInfo(p *StoredProfile, clientLibVersion uint16) i.A {
	accountType := 3
	if p.IsXbox {
		accountType = 9
	}
	platformUserID := strconv.FormatUint(p.PlatformUserID, 10)
	profileInfo := i.A{
		profileEntityVersion,
		p.ProfileID,
		getPlatformPath(p.IsXbox, p.PlatformUserID), // name
		p.AvatarMetadata, // metaData
		p.Alias,
	}
	if clientLibVersion >= i.V190 {
		profileInfo = append(profileInfo, p.Alias) // sanitizedAlias
	}
	profileInfo = append(
		profileInfo,
		"",             // clanName: empty = no clan
		p.StatGroupID,  // personalStatGroupID
		p.XP,           // xp
		p.Level,        // level
		p.Region,       // leaderboardRegionID
		nil,            // banInfo: not banned
		platformUserID, // platformUserID
		accountType,    // 3 = Steam, 9 = Xbox
		i.A{},          // linkedPlatformAccounts: empty = none
	)
	return profileInfo
}

func (u *MainUser) EncodePresence(definitions PresenceDefinitions) i.A {
	if definitions == nil || u.presenceProperties == nil {
		return i.A{}
	}
	presenceID := u.GetPresence()
	presenceDefinition := definitions.Get(presenceID)
	if presenceDefinition == nil {
		return i.A{}
	}
	var presencePropertyInfo i.A
	for id, value := range u.presenceProperties.Iter() {
		presencePropertyInfo = append(presencePropertyInfo, i.A{id, value})
	}

	return i.A{
		presenceID,
		(*presenceDefinition).GetLabel(), // presenceLocalized
		presencePropertyInfo,
	}
}

func (u *MainUser) GetPresence() int32 {
	return u.presence.Load()
}

func (u *MainUser) SetPresence(presence int32) {
	u.presence.Store(presence)
}

func (u *MainUser) SetPresenceProperty(id int32, value string) {
	if value == "" {
		u.presenceProperties.Delete(id)
	} else {
		u.presenceProperties.Store(id, value, func(_ string) bool {
			return true
		})
	}
}

func (u *MainUser) GetAvatarMetadata() *PersistentJsonData[*string] {
	return u.avatarMetadata
}

func (u *MainUser) GetPlatformRelated() uint8 {
	var value uint8
	if u.isXbox {
		value = 3
	}
	return value
}

func (u *MainUser) GetProfileLevel() uint16 {
	return 9_999
}

func (u *MainUser) GetProfileExperience() uint32 {
	return 0
}

func (u *MainUser) GetItems() *PersistentJsonData[*map[int32]*MainItem] {
	return u.items
}

func (u *MainUser) GetItemLoadouts() *PersistentJsonData[*MainItemLoadouts] {
	return u.itemLoadouts
}

func (u *MainUser) GetChallengeProgress() *PersistentJsonData[*[]ChallengeProgress] {
	return u.challengeProgress
}

func (u *MainUser) GetAuth() *PersistentJsonData[*time.Time] {
	return u.auth
}
