package router

import (
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// TestCommunityAchievements_DefensiveResourceShapes checks that a malformed
// achievement catalogue degrades to an empty list instead of panicking or
// emitting a half-built object. The resource is positional, so a server with the
// wrong file version must not take the public API down with it.
func TestCommunityAchievements_DefensiveResourceShapes(t *testing.T) {
	cases := map[string]i.A{
		"missing entries block": {0, "not-an-array", "signature"},
		"short entry":           {0, i.A{i.A{float64(1), "ONLY_TWO"}}, "signature"},
		"entry is not an array": {0, i.A{"nope"}, "signature"},
		"deleted flag as int":   {0, i.A{i.A{float64(1), "A", nil, nil, "", float64(1), float64(3), float64(-1)}}, "signature"},
		"no resource at all":    {0},
	}
	for name, resource := range cases {
		t.Run(name, func(t *testing.T) {
			g := newTestGame(t, game.AoE2)
			fake, ok := g.Resources().(*fakeResources)
			if !ok {
				t.Fatalf("resources are %T", g.Resources())
			}
			fake.arrayFiles["achievements.json"] = resource
			body := decodeBody(t, doCommunityRequest(t, (&Community{}).InitializeRoutes(game.AoE2, nil),
				"/community/achievement/getAvailableAchievements", g))
			defs, ok := body["achievementDefs"].([]any)
			if !ok {
				t.Fatalf("achievementDefs = %v, want an array", body["achievementDefs"])
			}
			wantLen := 0
			if name == "deleted flag as int" {
				// A non-zero deleted flag still drops the entry.
				wantLen = 0
			}
			if len(defs) != wantLen {
				t.Fatalf("achievementDefs = %v, want %d entries", defs, wantLen)
			}
		})
	}
}

// TestCommunityLeaderboards_DefensiveResourceShapes checks that a catalogue with
// missing or mistyped blocks still answers every block as an array.
func TestCommunityLeaderboards_DefensiveResourceShapes(t *testing.T) {
	cases := map[string]i.A{
		"too short":             {0, i.A{}, i.A{}},
		"blocks are not arrays": {0, "x", "y", "z", "w", "v", "u"},
		"rows are not arrays":   {0, i.A{"x"}, i.A{float64(1)}, i.A{true}, i.A{nil}, i.A{}, i.A{"y"}, i.A{}, i.A{}},
		"partial row":           {0, i.A{i.A{float64(1)}}, i.A{i.A{float64(1)}}, i.A{i.A{float64(1)}}, i.A{i.A{float64(1)}}, i.A{}, i.A{i.A{float64(1)}}, i.A{}, i.A{}},
	}
	for name, resource := range cases {
		t.Run(name, func(t *testing.T) {
			g := newTestGame(t, game.AoE2)
			fake, ok := g.Resources().(*fakeResources)
			if !ok {
				t.Fatalf("resources are %T", g.Resources())
			}
			fake.arrayFiles["leaderboards.json"] = resource
			body := decodeBody(t, doCommunityRequest(t, (&Community{}).InitializeRoutes(game.AoE2, nil),
				"/community/leaderboard/getAvailableLeaderboards", g))
			for _, key := range []string{"leaderboards", "matchTypes", "races", "factions", "leaderboardRegions"} {
				if _, ok := body[key].([]any); !ok {
					t.Errorf("%s = %v, want an array", key, body[key])
				}
			}
		})
	}
}
