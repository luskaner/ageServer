package community

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/achievement"
	"github.com/luskaner/ageServer/server/internal/routes/game/communityEvent"
	"github.com/luskaner/ageServer/server/internal/routes/game/leaderboard"
	"github.com/luskaner/ageServer/server/internal/routes/game/news"
)

func GetPersonalStat(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	ids := parseProfileIDs(r, "profile_ids")
	users := g.Users()
	for _, alias := range queryList(r, "aliases") {
		for userID := range users.GetUserIds() {
			if user, ok := users.GetUserById(userID); ok && strings.EqualFold(user.GetAlias(), alias) {
				ids = append(ids, user.GetId())
			}
		}
	}
	for _, path := range queryList(r, "profile_names") {
		if user, ok := userByPlatformPath(users, path); ok {
			ids = append(ids, user.GetId())
		}
	}
	ids = normalizeProfileIDs(ids)
	profiles := g.MatchHistory().GetProfiles(ids)
	ratings := g.MatchHistory().GetRatingsForLeaderboard(models.SoloRMRanked, ids)

	statGroups := internal.A{}
	profileInfo := internal.A{}
	stats := internal.A{}
	for _, id := range ids {
		user, online := users.GetUserById(id)
		stored := profiles[id]
		if !online && stored == nil {
			continue
		}
		statGroupID := id
		if online {
			statGroupID = user.GetStatGroupID()
			profileInfo = append(profileInfo, user.EncodeProfileInfo(internal.V190))
		} else {
			if stored.StatGroupID != 0 {
				statGroupID = stored.StatGroupID
			}
			profileInfo = append(profileInfo, models.EncodeStoredProfileInfo(stored, internal.V190))
		}
		rating := ratings[id]
		if rating != nil && rating.StatID != 0 && statGroupID == id {
			statGroupID = rating.StatID
		}
		statGroups = append(statGroups, internal.A{statGroupID, "", "", 1, internal.A{id}})
		var fallback int32
		if online {
			fallback = user.GetRating()
		} else {
			fallback = models.DefaultRating
		}
		stats = append(stats, models.EncodeLeaderboardStatFor(statGroupID, fallback, rating))
	}
	internal.JSON(&w, internal.A{0, statGroups, profileInfo, stats})
}

func GetAvatarStatForProfile(w http.ResponseWriter, r *http.Request) {
	users := models.G(r).Users()
	encoded := internal.A{}
	for _, path := range queryList(r, "profile_names") {
		if user, ok := userByPlatformPath(users, path); ok {
			encoded = append(encoded, user.EncodeStats()...)
		}
	}
	internal.JSON(&w, internal.A{0, encoded})
}

func GetAchievements(w http.ResponseWriter, r *http.Request) {
	ids := parseProfileIDs(r, "profileids")
	entries := make(internal.A, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, internal.A{id, internal.A{}})
	}
	internal.JSON(&w, internal.A{0, entries})
}

func FindClans(w http.ResponseWriter, _ *http.Request) {
	internal.JSON(&w, internal.A{0, internal.A{}})
}

func GetClanInfoFull(w http.ResponseWriter, _ *http.Request) {
	internal.JSON(&w, internal.A{0, internal.A{}})
}

func FindAdvertisements(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	profileIDs := make(map[int32]struct{})
	advertisements := g.Advertisements().LockedFindAdvertisementsEncoded(
		g.Title(),
		internal.V190,
		0,
		0,
		true,
		func(advertisement models.Advertisement) bool {
			if !advertisement.UnsafeGetVisible() || !advertisement.UnsafeGetJoinable() {
				return false
			}
			_, peerIDs := advertisement.GetPeers().Keys()
			for profileID := range peerIDs {
				profileIDs[profileID] = struct{}{}
			}
			return true
		},
	)
	ids := make([]int32, 0, len(profileIDs))
	for id := range profileIDs {
		ids = append(ids, id)
	}
	ids = normalizeProfileIDs(ids)
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	users := internal.A{}
	for _, id := range ids {
		if user, ok := g.Users().GetUserById(id); ok {
			users = append(users, user.EncodeProfileInfo(internal.V190))
		}
	}
	internal.JSON(&w, internal.A{0, advertisements, users})
}

func GetInventoryByProfileIDs(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	profileIDs := normalizeProfileIDs(parseProfileIDs(r, "profileids"))
	initialData := make(internal.A, len(profileIDs))
	finalData := make(internal.A, len(profileIDs))
	var locations internal.A
	if items := g.Items(); items != nil {
		locations = items.EncodeLocations()
	}
	for index, profileID := range profileIDs {
		items := internal.A{}
		if user, ok := g.Users().GetUserById(profileID); ok {
			userItems := user.GetItems()
			if userItems != nil {
				if err := userItems.WithReadOnly(func(data *map[int32]*models.MainItem) error {
					if data == nil {
						return nil
					}
					for _, item := range *data {
						if item != nil {
							items = append(items, item.Encode(profileID))
						}
					}
					return nil
				}); err != nil {
					logger.Printf("community.getInventoryByProfileIDs: profile %d: %s\n", profileID, err)
					http.Error(w, "could not read public profile inventory", http.StatusInternalServerError)
					return
				}
			}
		}
		id := strconv.FormatInt(int64(profileID), 10)
		initialData[index] = internal.A{id, items}
		finalData[index] = internal.A{id, locations}
	}
	internal.JSON(&w, internal.A{0, initialData, finalData})
}

func parseProfileIDs(r *http.Request, key string) []int32 {
	var ids []int32
	for _, value := range queryList(r, key) {
		id, err := strconv.ParseInt(value, 10, 32)
		if err == nil && id > 0 {
			ids = append(ids, int32(id))
		}
	}
	return ids
}

func queryList(r *http.Request, key string) []string {
	values := r.URL.Query()[key]
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		var decoded []string
		if json.Unmarshal([]byte(value), &decoded) == nil {
			result = append(result, decoded...)
			continue
		}
		value = strings.Trim(value, "[]")
		for _, part := range strings.Split(value, ",") {
			part = strings.Trim(strings.TrimSpace(part), `"'`)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func normalizeProfileIDs(ids []int32) []int32 {
	seen := make(map[int32]struct{}, len(ids))
	normalized := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func userByPlatformPath(users models.Users, path string) (models.User, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return nil, false
	}
	isXbox := strings.EqualFold(parts[0], "xbox")
	if !isXbox && !strings.EqualFold(parts[0], "steam") {
		return nil, false
	}
	platformID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, false
	}
	return users.GetUserByPlatformUserId(isXbox, platformID)
}

func Register(group interface {
	HandleFunc(method, path string, handler http.HandlerFunc)
}) {
	group.HandleFunc("GET", "/community/leaderboard/GetPersonalStat", GetPersonalStat)
	group.HandleFunc("GET", "/community/leaderboard/getRecentMatchHistory", leaderboard.GetCommunityRecentMatchHistory)
	group.HandleFunc("GET", "/community/advertisement/findAdvertisements", FindAdvertisements)
	group.HandleFunc("GET", "/community/leaderboard/GetAvatarStatForProfile", GetAvatarStatForProfile)
	group.HandleFunc("GET", "/community/achievement/getAchievements", GetAchievements)
	group.HandleFunc("GET", "/community/achievement/getAvailableAchievements", achievement.GetAvailableAchievements)
	group.HandleFunc("GET", "/community/news/getNews", news.GetNews)
	group.HandleFunc("GET", "/community/clan/find", FindClans)
	group.HandleFunc("GET", "/community/clan/getClanInfoFull", GetClanInfoFull)
	group.HandleFunc("GET", "/community/CommunityEvent/getAvailableCommunityEvents", communityEvent.GetAvailableCommunityEvents)
	group.HandleFunc("GET", "/community/item/getInventoryByProfileIDs", GetInventoryByProfileIDs)
	group.HandleFunc("GET", "/community/leaderboard/getAvailableLeaderboards", leaderboard.GetAvailableLeaderboards)
	group.HandleFunc("GET", "/community/leaderboard/getLeaderboard2", leaderboard.GetCommunityLeaderBoard)
	logger.Printf("Community API routes registered\n")
}
