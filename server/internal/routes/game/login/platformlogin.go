package login

import (
	"fmt"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/relationship"
	"github.com/luskaner/ageServer/server/internal/routes/wss"
)

type PlatformLoginRequest struct {
	AccountType      string `schema:"accountType"`
	PlatformUserId   uint64 `schema:"platformUserID"`
	Alias            string `schema:"alias"`
	GameId           string `schema:"title"`
	MacAddress       string `schema:"macAddress"`
	ClientLibVersion uint16 `schema:"clientLibVersion"`
}

// connectionID is the fixed value sent in the platformlogin response.
// The client parses but never uses this field; 0 is fine.
const connectionID = 0

func PlatformLoginError(t time.Time, w http.ResponseWriter) {
	i.JSON(&w, i.A{
		2,     // resultCode: 0 = success, 2 = error
		"",    // connectionToken
		0,     // connectionID
		t,     // serverTime
		i.A{}, // accountInfo
		i.A{}, // profiles
		0,     // banLevel
		0,     // banSeconds
		nil,   // banMessage
		nil,   // clientSettings
		i.A{}, // selectProfile
		i.A{}, // sslCertificates
		0,     // shouldUseClientSync: 0 = disabled (server-authoritative sync), 1 = enabled (client-side prediction)
		i.A{}, // relayServers
	})
}

func Platformlogin(w http.ResponseWriter, r *http.Request) {
	g := models.G(r)
	title := g.Title()
	users := g.Users()
	sessions := g.Sessions()
	u := r.Context().Value("user").(models.User)
	sess, ok := sessions.GetByUserId(u.GetId())
	if ok {
		sessions.Delete(sess.Id())
	}
	req := r.Context().Value("request").(PlatformLoginRequest)
	connectionToken := sessions.Create(u.GetId(), req.ClientLibVersion)
	sess, _ = sessions.GetById(connectionToken)
	presenceDefinitions := g.PresenceDefinitions()
	relationship.ChangePresence(req.ClientLibVersion, sessions, users, u, presenceDefinitions, 1)
	profile := u.EncodeProfileInfo(req.ClientLibVersion)
	if title == game.AoE3 || title == game.AoM || title == game.AoE4 {
		for user := range users.GetUserIds() {
			if user != u.GetId() {
				currentSess, currentOk := sessions.GetByUserId(user)
				if currentOk {
					wss.SendOrStoreMessage(
						currentSess,
						"FriendAcceptMessage",
						i.A{profile},
					)
				}
			}
		}
	}
	profileId := u.GetProfileId()
	leaderboardStat := i.A{}
	if title == game.AoE2 || i.SinceTheBalticPowers(title, req.ClientLibVersion) {
		var rating *models.PlayerRating
		if rows := g.MatchHistory().GetRatingsForLeaderboard(models.SoloRMRanked, []int32{u.GetId()}); rows != nil {
			rating = rows[u.GetId()]
		}
		leaderboardStat = append(leaderboardStat, u.EncodeLeaderboardStat(rating))
	}
	battleServers := g.BattleServers()
	relayServers := battleServers.Encode(r)
	if len(relayServers) == 0 {
		server := battleServers.NewBattleServer("")
		server.SetIPv4("127.0.0.1")
		server.SetBsPort(27012)
		server.SetWebSocketPort(27112)
		if title != game.AoE1 {
			server.SetName("localhost")
			server.SetOutOfBandPort(27212)
		}
		relayServers = append(relayServers, server.EncodeLogin(r))
	}
	serverTime := r.Context().Value("time").(time.Time).Unix()
	response := i.A{
		0, // resultCode: 0 = success, 2 = error
		connectionToken,
		connectionID,
		serverTime,
		i.A{
			profileId,           // id
			u.GetPlatformPath(), // accountName
			u.GetAccountType(),  // 3 = Steam, 9 = Xbox
			-1,                  // ssoID: not available
			0,                   // ssoStatus
			"en",                // country
			"eur",               // currency
			2,                   // spamAllowed: 0=none, 1=friends only, 2=all (default), 3=filtered
			nil,                 // metaData
			// profiles (optional) is omitted
		}, // accountInfo
		i.A{profile}, // profiles
		0,            // banLevel
		0,            // banSeconds
		nil,          // banMessage
	}
	var stats i.A
	if title == game.AoE1 {
		// AoE1 clients expect an extra top-level element before clientSettings.
		response = append(response, i.A{}) // extra (AoE1 only)
	} else {
		stats = u.EncodeStats()
	}
	selectProfile := i.A{
		0, // resultCode: 0 = success, 2 = error
		profile,
		// relationships
		relationship.Relationships(title, req.ClientLibVersion, users, u, presenceDefinitions),
		leaderboardStat,
		stats,
		nil,   // clanInfo
		i.A{}, // clanMembershipRequests
		nil,   // extendedData
		1,     // enableCrossplay: 1 = enabled
	}
	if title != game.AoE1 {
		selectProfile = append(selectProfile, i.A{}) // privacySettings
	}
	if req.ClientLibVersion >= i.V193 {
		selectProfile = append(selectProfile, -1) // extra field for newer clients
	}
	clientSettings := g.Resources().LoginData()
	response = append(response,
		clientSettings,
		selectProfile,
		i.A{}, // sslCertificates
		0,     // shouldUseClientSync: 0 = disabled (server-authoritative sync), 1 = enabled (client-side prediction)
		relayServers,
	)
	expiration := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	w.Header().Set("Set-Cookie", fmt.Sprintf("reliclink=%d; Expires=%s; Max-Age=3600", u.GetReliclink(), expiration))
	w.Header().Set("Request-Context", "appId=cid-v1:d21b644d-4116-48ea-a602-d6167fb46535")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	i.JSON(&w, response)
}
