package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// repoChallengesPath is the challenges.json of the repository, which the
// route tests serve so the real definitions are exercised.
var repoChallengesPath = filepath.Join("..", "..", "..", "resources", "responses", "age4", "challenges.json")

// challengesResource serves a fixed challenges.json. CreateMainGame calls
// Initialize on the resources again, which resets the in-memory assets of
// fakeResources, so the asset is installed here instead of before the game is
// built.
type challengesResource struct {
	*fakeResources
	challengesData []byte
}

func (r *challengesResource) Initialize(gameId string, opts *models.ResourcesOpts) {
	r.fakeResources.Initialize(gameId, opts)
	// challenges.json is an array asset in production, so install the real
	// definitions through the same accessor the server uses.
	var decoded i.A
	if err := json.Unmarshal(r.challengesData, &decoded); err != nil {
		panic(err)
	}
	r.fakeResources.arrayFiles["challenges.json"] = decoded
}

// newTestGameWithRealChallenges builds an AoE IV game whose resources serve the
// challenges.json of the repository.
func newTestGameWithRealChallenges(t *testing.T) (models.Game, string) {
	t.Helper()
	challengesData, err := os.ReadFile(repoChallengesPath)
	if err != nil {
		t.Fatalf("read %s: %v", repoChallengesPath, err)
	}
	origAuth := i.Authentication
	i.Authentication = "disabled"
	t.Cleanup(func() { i.Authentication = origAuth })
	origGen := i.GeneratePlatformUserId
	i.GeneratePlatformUserId = false
	t.Cleanup(func() { i.GeneratePlatformUserId = origGen })
	i.InitializeRng(42)
	resources := &challengesResource{
		fakeResources:  &fakeResources{},
		challengesData: challengesData,
	}
	g := models.CreateMainGame(game.AoE4, &models.CreateMainGameOpts{
		Instances: &models.InstanceOpts{Resources: resources},
	})
	return g, createSession(t, g)
}

// challengeProgressEntries decodes the [0, [[...], ...]] answer of the
// challenge progress routes.
func challengeProgressEntries(t *testing.T, body []byte) [][]any {
	t.Helper()
	var response []any
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("answer is not JSON: %v (%s)", err, body)
	}
	if len(response) != 2 {
		t.Fatalf("answer has %d elements, want 2: %s", len(response), body)
	}
	rawEntries, ok := response[1].([]any)
	if !ok {
		t.Fatalf("progress is not a list: %T", response[1])
	}
	entries := make([][]any, 0, len(rawEntries))
	for index, raw := range rawEntries {
		entry, ok := raw.([]any)
		if !ok {
			t.Fatalf("entry %d is not a positional array: %T", index, raw)
		}
		entries = append(entries, entry)
	}
	return entries
}

// The client renders a challenge as completed from the positional progress, so
// every challenge, including the Crucible masteries, must come back completed.
func TestAoE4GetChallengeProgressUnlocksEverything(t *testing.T) {
	g, sid := newTestGameWithRealChallenges(t)
	h := gameHandler(game.AoE4)
	rr := doRequest(h, "GET", "/game/Challenge/getChallengeProgress", g, sid, "", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	entries := challengeProgressEntries(t, rr.Body.Bytes())
	if len(entries) < 1000 {
		t.Fatalf("got %d progress entries, expected every definition", len(entries))
	}
	crucible := map[float64]bool{
		847: false, 849: false, 851: false, 529: false, 531: false,
		541: false, 543: false, 545: false, 549: false, 551: false,
		553: false, 555: false, 557: false, 559: false, 561: false,
	}
	for index, entry := range entries {
		if len(entry) != 7 {
			t.Fatalf("entry %d has %d fields, want 7: %v", index, len(entry), entry)
		}
		if entry[2].(float64) != 1 {
			t.Fatalf("entry %d is not completed: %v", index, entry)
		}
		if entry[3].(float64) != 0 {
			t.Fatalf("entry %d has a counter: %v", index, entry)
		}
		if entry[4] != nil {
			t.Fatalf("entry %d has an expiry instead of null: %v", index, entry)
		}
		if definitionID, ok := entry[1].(float64); ok {
			if _, isCrucible := crucible[definitionID]; isCrucible {
				crucible[definitionID] = true
			}
		}
	}
	for definitionID, found := range crucible {
		if !found {
			t.Errorf("Crucible mastery %v is missing", definitionID)
		}
	}
}

// The client reports progress through updateProgressBatched and expects a bare
// [0]; the progress it reports must not undo the unlocked state.
func TestAoE4UpdateProgressBatchedKeepsEverythingUnlocked(t *testing.T) {
	g, sid := newTestGameWithRealChallenges(t)
	h := gameHandler(game.AoE4)
	rr := doRequest(h, "POST", "/game/challenge/updateProgressBatched", g, sid,
		"progressIDs=%5B1%2C2%5D&updateAmounts=%5B1%2C1%5D", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var answer []any
	if err := json.Unmarshal(rr.Body.Bytes(), &answer); err != nil {
		t.Fatalf("answer is not JSON: %v (%s)", err, rr.Body.String())
	}
	if len(answer) != 1 || answer[0].(float64) != 0 {
		t.Fatalf("answer is %s, want [0]", rr.Body.String())
	}
	rr = doRequest(h, "GET", "/game/Challenge/getChallengeProgress", g, sid, "", nil)
	for index, entry := range challengeProgressEntries(t, rr.Body.Bytes()) {
		if entry[2].(float64) != 1 || entry[3].(float64) != 0 {
			t.Fatalf("entry %d was reverted by the update: %v", index, entry)
		}
	}
}

// getChallengeProgressByProfileID must answer with the progress of the
// requested profile, not with the progress of the caller.
func TestAoE4GetChallengeProgressByProfileID(t *testing.T) {
	g, sid := newTestGameWithRealChallenges(t)
	h := gameHandler(game.AoE4)
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	user, ok := g.Users().GetUserById(sess.GetUserId())
	if !ok {
		t.Fatal("session user is missing")
	}
	rr := doRequest(h, "GET", "/game/Challenge/getChallengeProgressByProfileID?profile_id=1", g, sid, "", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if entries := challengeProgressEntries(t, rr.Body.Bytes()); len(entries) != 0 {
		t.Fatalf("unknown profile answered %d entries", len(entries))
	}
	rr = doRequest(h, "GET",
		"/game/Challenge/getChallengeProgressByProfileID?profile_id="+strconv.FormatInt(int64(user.GetProfileId()), 10),
		g, sid, "", nil)
	entries := challengeProgressEntries(t, rr.Body.Bytes())
	if len(entries) < 1000 {
		t.Fatalf("known profile answered %d entries", len(entries))
	}
	for index, entry := range entries {
		if entry[2].(float64) != 1 {
			t.Fatalf("entry %d is not completed: %v", index, entry)
		}
	}
}
