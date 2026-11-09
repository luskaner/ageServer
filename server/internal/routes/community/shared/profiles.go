package shared

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/luskaner/ageServer/server/internal/models"
)

// platformPathPattern is the only accepted form of profile_names. The official
// API receives the same /steam/<id> and /xboxlive/<id> paths it hands out as the
// profile "name" field.
//
// This is input normalisation, not path-traversal defence: the value is parsed
// to an integer and used as a lookup key, never concatenated into a file path,
// and /community never calls GetOrCreateUser (which is the only code that
// derives file names). See §1.5.
var platformPathPattern = regexp.MustCompile(`^/(steam|xboxlive)/([0-9]+)$`)

// PlatformPath is a parsed profile_names entry.
type PlatformPath struct {
	PlatformUserID uint64
	IsXbox         bool
}

// ParsePlatformPath parses /steam/<id> or /xboxlive/<id>. Anything else (an
// unsupported prefix, a non-numeric id, a traversal attempt, an over-long
// value) returns false and is dropped by the caller.
func ParsePlatformPath(value string) (PlatformPath, bool) {
	if len(value) > MaxQueryString {
		return PlatformPath{}, false
	}
	match := platformPathPattern.FindStringSubmatch(value)
	if match == nil {
		return PlatformPath{}, false
	}
	id, err := strconv.ParseUint(match[2], 10, 64)
	if err != nil || id == 0 {
		return PlatformPath{}, false
	}
	return PlatformPath{PlatformUserID: id, IsXbox: match[1] == "xboxlive"}, true
}

// Profile is one profile as the public API renders it, in any of the places a
// profile appears: statGroups[].members[], profiles[], avatars[]. It is a plain
// value so an offline profile (only known through the stored identity) and a
// connected one (a live models.User) travel through the same type, and it is the
// wire format itself, so handlers embed it instead of re-encoding a map (§3.2).
//
// It must not be confused with models.EncodeProfileInfo, which is the positional
// game-client layout and lacks country and clanlist_name.
type Profile struct {
	ProfileID int32 `json:"profile_id"`
	// Username is the platform identity, /steam/<id> or /xboxlive/<id>.
	Username string `json:"name"`
	Alias    string `json:"alias"`
	// StatGroupID is the stat group the profile plays under; the official API
	// reports it as an integer and 0 when unknown.
	StatGroupID int32  `json:"personal_statgroup_id"`
	XP          uint32 `json:"xp"`
	Level       uint16 `json:"level"`
	Region      uint8  `json:"leaderboardregion_id"`
	// Country has no source in this server: the stored profile table has no
	// country column, so the field is served empty rather than omitted (§3.2).
	Country string `json:"country"`
	// ClanListName is always empty because there is no clan model (§8.1), which
	// matches the clanName="" the game client already receives.
	ClanListName string `json:"clanlist_name"`

	// User is set only when the profile is connected right now. Endpoints that
	// need live-only data (avatar stats) check it for nil. It is not on the wire.
	User models.User `json:"-"`
}

// ProfileRequest carries every spelling of profile identification the endpoints
// accept. The spellings are not uniform in the official API and must not be
// unified (§3.2): profileids, profile_ids, profile_names, profileNames and
// aliases each appear on a fixed set of endpoints.
type ProfileRequest struct {
	ProfileIDs []int32
	Names      []string
	Aliases    []string
}

// ResolveProfiles turns a ProfileRequest into the ordered, de-duplicated list of
// profiles it names.
//
// Precedence per id is the one the game routes already use (getLeaderBoard.go):
// the connected user first, then the stored profile, then a placeholder. Doing
// it the other way round would leave connected players without alias or XP,
// which is the trap the simplified flow of correction 2 fell into (§3.2).
func ResolveProfiles(r *http.Request, req ProfileRequest) []*Profile {
	game := models.G(r)
	users := game.Users()
	history := game.MatchHistory()

	seen := make(map[int32]struct{}, len(req.ProfileIDs))
	resolved := make([]*Profile, 0, len(req.ProfileIDs))

	addProfile := func(profile *Profile) {
		if profile == nil {
			return
		}
		if _, duplicate := seen[profile.ProfileID]; duplicate {
			return
		}
		seen[profile.ProfileID] = struct{}{}
		resolved = append(resolved, profile)
	}

	addStored := func(profileID int32) {
		if profileID <= 0 {
			return
		}
		if _, duplicate := seen[profileID]; duplicate {
			return
		}
		stored := history.GetProfiles([]int32{profileID})[profileID]
		if stored == nil {
			stored = models.PlaceholderProfile(profileID, 0)
		}
		addProfile(profileFromStored(stored))
	}

	for _, profileID := range req.ProfileIDs {
		if profileID <= 0 {
			continue
		}
		if _, duplicate := seen[profileID]; duplicate {
			continue
		}
		if user, connected := users.GetUserById(profileID); connected {
			addProfile(profileFromUser(user))
			continue
		}
		addStored(profileID)
	}

	// Names are resolved through the live user first and the stored profile
	// second, exactly like an id.
	for _, name := range req.Names {
		path, ok := ParsePlatformPath(name)
		if !ok {
			continue
		}
		if user, connected := users.GetUserByPlatformUserId(path.IsXbox, path.PlatformUserID); connected {
			addProfile(profileFromUser(user))
			continue
		}
		if stored, found := history.GetProfileByPlatformUserID(path.PlatformUserID, path.IsXbox); found {
			addStored(stored.ProfileID)
		}
	}

	// Aliases only exist for connected users: the stored identity is keyed by
	// platform account, not by alias.
	for _, alias := range req.Aliases {
		alias = strings.TrimSpace(SanitizeString(alias))
		if alias == "" {
			continue
		}
		user, connected := users.GetUserByAlias(alias)
		if !connected {
			continue
		}
		addProfile(profileFromUser(user))
	}
	return resolved
}

func profileFromUser(user models.User) *Profile {
	return &Profile{
		ProfileID:   user.GetId(),
		Username:    user.GetPlatformPath(),
		Alias:       user.GetAlias(),
		StatGroupID: user.GetStatGroupID(),
		XP:          user.GetProfileExperience(),
		Level:       user.GetProfileLevel(),
		Region:      user.GetPlatformRelated(),
		User:        user,
	}
}

func profileFromStored(stored *models.StoredProfile) *Profile {
	return &Profile{
		ProfileID:   stored.ProfileID,
		Username:    models.StoredProfilePlatformPath(stored),
		Alias:       stored.Alias,
		StatGroupID: stored.StatGroupID,
		XP:          stored.XP,
		Level:       stored.Level,
		Region:      stored.Region,
	}
}

// ProfileIDs returns the ids of the resolved profiles, in order. Endpoints that
// query the ladder store need the plain id list.
func ProfileIDs(profiles []*Profile) []int32 {
	ids := make([]int32, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ProfileID)
	}
	return ids
}

// StatGroupIDOf resolves the stat group id used to join a ladder row with its
// profile. It follows models.ResolveStatGroupID with the same precedence the
// game routes use, falling back to the profile id so the join always has a key.
func StatGroupIDOf(profile *Profile, rating *models.PlayerRating) int32 {
	var stored, ratingStatID int32
	if profile != nil {
		stored = profile.StatGroupID
	}
	if rating != nil {
		ratingStatID = rating.StatID
	}
	statGroupID := models.ResolveStatGroupID(0, stored, ratingStatID)
	if statGroupID == 0 && profile != nil {
		statGroupID = profile.ProfileID
	}
	return statGroupID
}

// StatGroup is the statGroups[] entry that pairs a profile with its ladder rows.
// Clients join it to leaderboardStats[] by statgroup_id, never by index (C17).
type StatGroup struct {
	ID      int32      `json:"id"`
	Name    string     `json:"name"`
	Type    int        `json:"type"`
	Members []*Profile `json:"members"`
}

// NewStatGroup builds the single-member stat group of a profile. This server has
// no clan stat groups, so every group is type 1 (player) with an empty name.
func NewStatGroup(profile *Profile, statGroupID int32) StatGroup {
	return StatGroup{
		ID:      statGroupID,
		Name:    "",
		Type:    1,
		Members: []*Profile{profile},
	}
}
