// Package achievement implements the two public achievement endpoints.
package achievement

import (
	"net/http"
	"strconv"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/community/shared"
)

// Definition is one entry of the achievement catalogue (8 fields).
type Definition struct {
	ID                     any    `json:"id"`
	Name                   string `json:"name"`
	NameLocStringID        int64  `json:"name_locstringid"`
	DescriptionLocStringID int64  `json:"description_locstringid"`
	Icon                   string `json:"icon"`
	TimeModified           int64  `json:"timemodified"`
	Deleted                int64  `json:"deleted"`
	ItemBundleID           int64  `json:"itembundle_id"`
}

// CatalogueResponse is the body of getAvailableAchievements.
type CatalogueResponse struct {
	Result          shared.Result `json:"result"`
	AchievementDefs []Definition  `json:"achievementDefs"`
}

// GetAchievementsResponse is the body of getAchievements.
//
// The map is keyed by profile id, so this is one of the few responses that stays
// map-based: the keys are caller-supplied and cannot be struct fields.
type GetAchievementsResponse struct {
	Result              shared.Result         `json:"result"`
	UserAchievementsMap []map[string][]Earned `json:"userAchievementsMap"`
}

// Earned is one earned achievement. Nothing ever fills it today: see the comment
// on GetAchievements.
type Earned struct {
	AchievementID int32 `json:"achievement_id"`
	AchievedDate  int64 `json:"achieveddate"`
}

// GetAvailableAchievements returns the achievement catalogue of the title.
//
// Source: resources/responses/<game>/achievements.json, a positional
// [code, [[8 fields]...], signature] document. It is already loaded into
// arrayFiles for every title that ships it, so no resource change is needed
// (§1.c, C1). age1 has no such file: it answers with an empty list.
func GetAvailableAchievements(w http.ResponseWriter, r *http.Request) {
	game := models.G(r)
	resource := game.Resources().ArrayFiles()["achievements.json"]
	response := CatalogueResponse{Result: shared.OK(), AchievementDefs: make([]Definition, 0)}
	// The catalogue lives in the second slot; the first is the result code and
	// the last is the signature (which the public API does not forward).
	for _, raw := range shared.RowsAt(resource, 1) {
		entry, ok := raw.(i.A)
		if !ok {
			continue
		}
		definition, ok := encodeDefinition(entry)
		if !ok {
			continue
		}
		response.AchievementDefs = append(response.AchievementDefs, definition)
	}
	i.JSON(&w, response)
}

// GetAchievements returns an empty achievement list per requested profile.
//
// The lists are empty on purpose. Serving real achievements to the game client
// grants them on Xbox (see routes/game/achievement/getAchievements.go) and this
// server does not persist progress at all. The public endpoint does not inherit
// that risk, but there is nothing to report either way, so the shape is served
// with empty arrays instead of the endpoint being omitted (§3.a, §8.2).
//
// The key is the profile id without quotes, which is what the live API sends and
// the opposite of the inventory endpoint.
func GetAchievements(w http.ResponseWriter, r *http.Request) {
	var query shared.ProfileQuery
	shared.BindQuery(r, &query)
	ids := query.ProfileRequest().ProfileIDs
	response := GetAchievementsResponse{
		Result:              shared.OK(),
		UserAchievementsMap: make([]map[string][]Earned, 0, len(ids)),
	}
	for _, id := range ids {
		response.UserAchievementsMap = append(response.UserAchievementsMap,
			map[string][]Earned{strconv.FormatInt(int64(id), 10): {}})
	}
	i.JSON(&w, response)
}

// encodeDefinition maps one positional catalogue row to its object form. Rows
// shorter than the 8 documented fields are rejected rather than half-filled.
//
// The two locstring id slots are title dependent: age2 stores null there and age4
// stores the display text itself. Neither is a localisation id, so both are
// reported as -1, the documented "no localisation" sentinel, instead of leaking a
// null or a string into a field the contract declares as an integer.
func encodeDefinition(entry i.A) (Definition, bool) {
	if len(entry) < 8 {
		return Definition{}, false
	}
	// Entries flagged as deleted are skipped: the official service stops
	// publishing them without removing the row.
	deleted, _ := shared.Int64At(entry, 6)
	if deleted != 0 {
		return Definition{}, false
	}
	nameLoc := int64(-1)
	if id, ok := shared.Int64At(entry, 2); ok {
		nameLoc = id
	}
	descriptionLoc := int64(-1)
	if id, ok := shared.Int64At(entry, 3); ok {
		descriptionLoc = id
	}
	timeModified, _ := shared.Int64At(entry, 5)
	itemBundle := int64(-1)
	if id, ok := shared.Int64At(entry, 7); ok {
		itemBundle = id
	}
	return Definition{
		ID:                     shared.At(entry, 0),
		Name:                   shared.StringAt(entry, 1),
		NameLocStringID:        nameLoc,
		DescriptionLocStringID: descriptionLoc,
		Icon:                   shared.StringAt(entry, 4),
		TimeModified:           timeModified,
		Deleted:                deleted,
		ItemBundleID:           itemBundle,
	}, true
}
