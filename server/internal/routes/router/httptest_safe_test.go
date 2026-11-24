package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// useSafeMode installs a Safe configuration for the test and restores the
// previous one when it ends.
func useSafeMode(t *testing.T, mode string, exceptions ...string) {
	t.Helper()
	old := i.SafeConfigRuntime
	t.Cleanup(func() { i.SafeConfigRuntime = old })
	i.SafeConfigRuntime.Enabled = mode
	i.SafeConfigRuntime.PlatformUserIdExceptions = exceptions
}

// achievementIDs decodes [0, [[profileId, [[id, date], ...]]]] into the ids.
func achievementIDs(t *testing.T, body []byte) []float64 {
	t.Helper()
	var response []any
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("answer is not JSON: %v (%s)", err, body)
	}
	outer, ok := response[1].([]any)
	if !ok || len(outer) != 1 {
		t.Fatalf("answer has no profile entry: %s", body)
	}
	profile, ok := outer[0].([]any)
	if !ok || len(profile) != 2 {
		t.Fatalf("profile entry is malformed: %s", body)
	}
	rawEarned, ok := profile[1].([]any)
	if !ok {
		t.Fatalf("earned achievements are %T, want a list", profile[1])
	}
	ids := make([]float64, 0, len(rawEarned))
	for index, raw := range rawEarned {
		entry, ok := raw.([]any)
		if !ok || len(entry) != 2 {
			t.Fatalf("achievement %d is malformed: %v", index, raw)
		}
		id, ok := entry[0].(float64)
		if !ok {
			t.Fatalf("achievement %d id is %T", index, entry[0])
		}
		ids = append(ids, id)
	}
	return ids
}

func getAchievements(t *testing.T, h http.Handler, g models.Game, sid string) []float64 {
	t.Helper()
	rr := doRequest(h, "GET", "/game/Achievement/getAchievements", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	return achievementIDs(t, rr.Body.Bytes())
}

func TestSafeModeAchievements(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	resources, ok := g.Resources().(*fakeResources)
	if !ok {
		t.Fatalf("resources are %T, want *fakeResources", g.Resources())
	}
	resources.arrayFiles["achievements.json"] = i.A{0, i.A{
		i.A{1, "a", "A", "", "", 1700000000, 0, -1},
		i.A{2, "b", "B", "", "", 0, 0, -1},
		i.A{3, "c", "C", "", "", 1700000001, 1, -1},
	}, "fake"}
	sid := createSession(t, g)
	h := gameHandler(game.AoE2)
	useSafeMode(t, i.SafeFalse)

	if got := getAchievements(t, h, g, sid); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("unsafe achievements = %v, want [1 2]", got)
	}

	i.SafeConfigRuntime.Enabled = i.SafeTrue
	if got := getAchievements(t, h, g, sid); len(got) != 0 {
		t.Fatalf("safe achievements = %v, want none", got)
	}

	i.SafeConfigRuntime.Enabled = i.SafeAuto
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	sess.SetOfficial(false)
	if got := getAchievements(t, h, g, sid); len(got) != 2 {
		t.Fatalf("auto/emulated achievements = %v, want [1 2]", got)
	}
	sess.SetOfficial(true)
	if got := getAchievements(t, h, g, sid); len(got) != 0 {
		t.Fatalf("auto/official achievements = %v, want none", got)
	}

	// An exception inverts the resolved value, exactly like [Sign].
	i.SafeConfigRuntime.PlatformUserIdExceptions = []string{"76561198000000001"}
	if got := getAchievements(t, h, g, sid); len(got) != 2 {
		t.Fatalf("auto/official/exception achievements = %v, want [1 2]", got)
	}
}

func challengeEntries(t *testing.T, h http.Handler, g models.Game, sid string) [][]any {
	t.Helper()
	rr := doRequest(h, "GET", "/game/Challenge/getChallengeProgress", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	return challengeProgressEntries(t, rr.Body.Bytes())
}

func TestSafeModeChallengeProgress(t *testing.T) {
	g, sid := newTestGameWithRealChallenges(t)
	h := gameHandler(game.AoE4)
	useSafeMode(t, i.SafeTrue)

	entries := challengeEntries(t, h, g, sid)
	if len(entries) < 1000 {
		t.Fatalf("safe progress has %d entries, want every definition", len(entries))
	}
	for index, entry := range entries {
		if len(entry) != 7 {
			t.Fatalf("entry %d has %d fields, want 7", index, len(entry))
		}
		if entry[2].(float64) != 0 || entry[3].(float64) != 0 || entry[5] != nil {
			t.Fatalf("entry %d is not pending: %v", index, entry)
		}
	}

	i.SafeConfigRuntime.Enabled = i.SafeFalse
	entries = challengeEntries(t, h, g, sid)
	for index, entry := range entries {
		if entry[2].(float64) != 1 {
			t.Fatalf("unsafe entry %d is not completed: %v", index, entry)
		}
	}
}

func TestSafeModeChallengeUpdatesAreNotPersisted(t *testing.T) {
	g, sid := newTestGameWithRealChallenges(t)
	h := gameHandler(game.AoE4)
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	user, found := g.Users().GetUserById(sess.GetUserId())
	if !found {
		t.Fatal("session user is missing")
	}
	if user.GetChallengeProgress() == nil {
		t.Fatal("user has no challenge progress")
	}
	var progressID int64
	if err := user.GetChallengeProgress().WithReadWrite(func(data *[]models.ChallengeProgress) error {
		if len(*data) == 0 {
			t.Fatal("user has no challenge records")
		}
		record := (*data)[0]
		record[models.ChallengeProgressFieldCompletionCount] = int64(0)
		record[models.ChallengeProgressFieldProgress] = int64(0)
		record[models.ChallengeProgressFieldLastCompletedTime] = nil
		progressID, _ = models.ChallengeProgressID(record)
		return nil
	}); err != nil {
		t.Fatalf("prepare pending challenge: %v", err)
	}
	if progressID == 0 {
		t.Fatal("the first challenge has no progress ID")
	}
	useSafeMode(t, i.SafeTrue)
	body := "progressIDs=%5B" + strconv.FormatInt(progressID, 10) + "%5D&updateAmounts=%5B1000000%5D"
	rr := doRequest(h, "POST", "/game/challenge/updateProgressBatched", g, sid, body, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("safe update status %d: %s", rr.Code, rr.Body.String())
	}
	completed := func() bool {
		var value bool
		_ = user.GetChallengeProgress().WithReadOnly(func(data *[]models.ChallengeProgress) error {
			for _, record := range *data {
				if id, ok := models.ChallengeProgressID(record); ok && id == progressID {
					value = models.ChallengeProgressCompleted(record)
				}
			}
			return nil
		})
		return value
	}
	if completed() {
		t.Fatal("a safe update persisted the challenge completion")
	}

	i.SafeConfigRuntime.Enabled = i.SafeFalse
	rr = doRequest(h, "POST", "/game/challenge/updateProgressBatched", g, sid, body, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("unsafe update status %d: %s", rr.Code, rr.Body.String())
	}
	if !completed() {
		t.Fatal("an unsafe update did not complete the challenge")
	}
}

func avatarStatValue(t *testing.T, user models.User, id int32) int64 {
	t.Helper()
	var value int64
	if err := user.GetAvatarStats().WithReadOnly(func(stats *models.AvatarStats) error {
		if stat, ok := stats.GetStat(id); ok {
			value = stat.Value
		}
		return nil
	}); err != nil {
		t.Fatalf("read avatar stat: %v", err)
	}
	return value
}

func TestSafeModeBlocksAvatarStatWrites(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	sid := createSession(t, g)
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	user, found := g.Users().GetUserById(sess.GetUserId())
	if !found {
		t.Fatal("session user is missing")
	}
	if user.GetAvatarStats() == nil {
		t.Skip("this game has no avatar stats store")
	}
	h := gameHandler(game.AoE2)
	useSafeMode(t, i.SafeFalse)

	rr := doRequest(h, "POST", "/game/leaderboard/setAvatarStatValues", g, sid,
		"avatarStat_ids=%5B1%5D&values=%5B10%5D&updateTypes=%5B1%5D", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("unsafe status %d: %s", rr.Code, rr.Body.String())
	}
	if got := avatarStatValue(t, user, 1); got != 10 {
		t.Fatalf("unsafe value = %d, want 10", got)
	}

	i.SafeConfigRuntime.Enabled = i.SafeTrue
	rr = doRequest(h, "POST", "/game/leaderboard/setAvatarStatValues", g, sid,
		"avatarStat_ids=%5B1%5D&values=%5B99%5D&updateTypes=%5B1%5D", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("safe status %d: %s", rr.Code, rr.Body.String())
	}
	if got := avatarStatValue(t, user, 1); got != 10 {
		t.Fatalf("safe value = %d, want the previous 10", got)
	}
}

func TestPlatformUserIdForRequestUsesSession(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	sid := createSession(t, g)
	sess, found := g.Sessions().GetById(sid)
	if !found {
		t.Fatal("session is missing")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), models.GameContextKey, g)
	ctx = context.WithValue(ctx, "session", sess)
	req = req.WithContext(ctx)
	if got := models.PlatformUserIdForRequest(req); got != 76561198000000001 {
		t.Fatalf("platform user id = %d, want 76561198000000001", got)
	}
}
