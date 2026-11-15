package router

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// rewardItemsDefinitions declares one reward item (is_new in the definition)
// and one plain item, which is what the client uses to decide whether to show
// the new badge.
const rewardItemsDefinitions = `{
	"itemCategories": [{"categoryID": 1, "name": "Portrait", "categoryGroup": "Portrait"}],
	"itemDefinitions": [
		{
			"id": 1001,
			"name": "reward",
			"categoryIDs": [1],
			"metaData": "{\"att\":{\"is_new\":{\"val\":\"1\",\"source\":1,\"mtype\":1}},\"icon\":\"x\"}"
		},
		{
			"id": 1002,
			"name": "plain",
			"categoryIDs": [1],
			"metaData": "{\"icon\":\"y\"}"
		}
	]
}`

// rewardItemsResource serves the reward item definitions. CreateMainGame calls
// Initialize again, which resets the in-memory assets of fakeResources, so the
// definitions are installed here instead of before the game is built.
type rewardItemsResource struct {
	*fakeResources
}

func (r *rewardItemsResource) Initialize(gameId string, opts *models.ResourcesOpts) {
	r.fakeResources.Initialize(gameId, opts)
	r.fakeResources.signedAssets["itemDefinitions.json"] = []byte(rewardItemsDefinitions)
}

// newTestGameWithRewardItems builds an AoE IV game whose resources serve the
// reward item definitions, with a fresh profile so the items are the ones this
// test defines.
func newTestGameWithRewardItems(t *testing.T) (models.Game, string) {
	t.Helper()
	origAuth := i.Authentication
	i.Authentication = "disabled"
	t.Cleanup(func() { i.Authentication = origAuth })
	origGen := i.GeneratePlatformUserId
	i.GeneratePlatformUserId = false
	t.Cleanup(func() { i.GeneratePlatformUserId = origGen })
	i.InitializeRng(42)
	resources := &rewardItemsResource{fakeResources: &fakeResources{}}
	g := models.CreateMainGame(game.AoE4, &models.CreateMainGameOpts{
		Instances: &models.InstanceOpts{Resources: resources},
	})
	// A platform user ID of its own keeps the profile away from the one the
	// other router tests share, whose items were created from other
	// definitions.
	const platformUserID uint64 = 76561198999999998
	dir := models.UserDataDir(game.AoE4, true, strconv.FormatUint(platformUserID, 10))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	user := g.Users().GetOrCreateUser(
		game.AoE4,
		g.Items(),
		g.LeaderboardDefinitions().AvatarStatDefinitions(),
		"127.0.0.1:1234",
		"00:11:22:33:44:55",
		false,
		platformUserID,
		"rewarduser",
	)
	if user == nil {
		t.Fatal("failed to create user")
	}
	return g, g.Sessions().Create(user.GetId(), 200)
}

// sessionItems returns the encoded items of the session's profile, keyed by
// item definition ID.
func sessionItems(t *testing.T, g models.Game, sid string) map[float64][]any {
	t.Helper()
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	user, ok := g.Users().GetUserById(sess.GetUserId())
	if !ok {
		t.Fatal("session user is missing")
	}
	items := map[float64][]any{}
	if err := user.GetItems().WithReadOnly(func(data *map[int32]*models.MainItem) error {
		for _, item := range *data {
			encoded := item.Encode(user.GetProfileId())
			definitionID, ok := encoded[2].(int32)
			if !ok {
				continue
			}
			items[float64(definitionID)] = encoded
		}
		return nil
	}); err != nil {
		t.Fatalf("read items: %s", err)
	}
	return items
}

// metadataAttribute reads an attribute value out of the encoded item metadata.
func metadataAttribute(t *testing.T, item []any, key string) (string, bool) {
	t.Helper()
	raw, ok := item[6].(string)
	if !ok {
		t.Fatalf("item metadata is %T", item[6])
	}
	var metadata struct {
		Att map[string]struct {
			Val string `json:"val"`
		} `json:"att"`
	}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		t.Fatalf("item metadata %q is not JSON: %s", raw, err)
	}
	attr, found := metadata.Att[key]
	return attr.Val, found
}

// The client clears the new badge through updateItemAttributes. The official
// service accepts it and stores it on the item, so the server must not answer
// with an error code: otherwise the badge never goes away.
func TestAoE4UpdateItemAttributesClearsIsNew(t *testing.T) {
	g, sid := newTestGameWithRewardItems(t)
	h := gameHandler(game.AoE4)
	items := sessionItems(t, g, sid)
	reward, ok := items[1001]
	if !ok {
		t.Fatal("reward item was not granted")
	}
	// Items handed out by default must already be seen.
	if value, found := metadataAttribute(t, reward, "is_new"); !found || value != "0" {
		t.Fatalf("default reward item is_new = %q,%v; want 0,true", value, found)
	}

	// The client asks to clear it again (idempotent) and expects code 0.
	itemID := reward[0].(int32)
	body := "attributeKeys=%5B%5B%22is_new%22%5D%5D" +
		"&attributeValues=%5B%5B%220%22%5D%5D" +
		"&itemInstance_ids=%5B" + strconv.FormatInt(int64(itemID), 10) + "%5D" +
		"&xpGains=%5B0%5D"
	rr := doRequest(h, "POST", "/game/item/updateItemAttributes", g, sid, body, nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var answer []any
	if err := json.Unmarshal(rr.Body.Bytes(), &answer); err != nil {
		t.Fatalf("answer is not JSON: %v (%s)", err, rr.Body.String())
	}
	if len(answer) != 3 || answer[0].(float64) != 0 {
		t.Fatalf("answer = %s, want [0, ...]", rr.Body.String())
	}
	codes, ok := answer[1].([]any)
	if !ok || len(codes) != 1 {
		t.Fatalf("error codes = %#v", answer[1])
	}
	first, ok := codes[0].([]any)
	if !ok || len(first) != 2 || first[0].(float64) != 0 {
		t.Fatalf("item error code = %#v, want [0, [0]]", codes[0])
	}
	encoded, ok := answer[2].([]any)
	if !ok || len(encoded) != 1 {
		t.Fatalf("encoded items = %#v", answer[2])
	}
	updated, ok := encoded[0].([]any)
	if !ok {
		t.Fatalf("encoded item = %#v", encoded[0])
	}
	if value, found := metadataAttribute(t, updated, "is_new"); !found || value != "0" {
		t.Fatalf("updated item is_new = %q,%v; want 0,true", value, found)
	}

	// The flag is persisted, not only echoed.
	if value, found := metadataAttribute(t, sessionItems(t, g, sid)[1001], "is_new"); !found || value != "0" {
		t.Fatalf("persisted is_new = %q,%v; want 0,true", value, found)
	}
}

// A plain item (its definition does not declare is_new) must not get the
// attribute, matching the official inventory.
func TestAoE4DefaultPlainItemHasNoIsNew(t *testing.T) {
	g, sid := newTestGameWithRewardItems(t)
	items := sessionItems(t, g, sid)
	plain, ok := items[1002]
	if !ok {
		t.Fatal("plain item was not granted")
	}
	if value, found := metadataAttribute(t, plain, "is_new"); found {
		t.Fatalf("plain item is_new = %q; want absent", value)
	}
}
