package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal/routes/game/login"
)

func TestPlatformloginStoresOfficialFlag(t *testing.T) {
	cases := []struct {
		name         string
		xbox         bool
		plainSteam   bool
		wantOfficial bool
	}{
		{"xbox is always official", true, true, true},
		{"steam plaintext ticket is not official", false, true, false},
		{"steam encrypted ticket is official", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGame(t, game.AoE2)
			u := g.Users().GetOrCreateUser(
				g.Title(),
				g.Items(),
				g.LeaderboardDefinitions().AvatarStatDefinitions(),
				"127.0.0.1:1",
				"00:11:22:33:44:55",
				tc.xbox,
				76561198000000123,
				"official",
			)
			if u == nil {
				t.Fatal("user was not created")
			}
			u.SetSteamTicketPlaintext(tc.plainSteam)

			accountType := "STEAM"
			if tc.xbox {
				accountType = "XBOXLIVE"
			}
			body := "accountType=" + accountType +
				"&platformUserID=76561198000000123&alias=official&title=age2" +
				"&macAddress=00:11:22:33:44:55&clientLibVersion=200"
			req := httptest.NewRequest("POST", "/game/login/platformlogin", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			ctx := context.WithValue(req.Context(), "game", g)
			ctx = context.WithValue(ctx, "user", u)
			ctx = context.WithValue(ctx, "request", login.PlatformLoginRequest{
				AccountType:      accountType,
				PlatformUserId:   76561198000000123,
				Alias:            "official",
				GameId:           "age2",
				MacAddress:       "00:11:22:33:44:55",
				ClientLibVersion: 200,
			})
			ctx = context.WithValue(ctx, "time", time.Now().UTC())

			rr := httptest.NewRecorder()
			login.Platformlogin(rr, req.WithContext(ctx))

			if rr.Code != 200 {
				t.Fatalf("platformlogin status = %d, body = %s", rr.Code, rr.Body.String())
			}
			sess, ok := g.Sessions().GetByUserId(u.GetId())
			if !ok {
				t.Fatal("session was not created")
			}
			if got := sess.Official(); got != tc.wantOfficial {
				t.Fatalf("session official = %v, want %v", got, tc.wantOfficial)
			}
		})
	}
}
