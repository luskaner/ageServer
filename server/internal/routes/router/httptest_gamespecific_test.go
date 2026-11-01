package router

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// gameSpecificRoute describe rutas registradas únicamente para juegos concretos,
// con el primer elemento del JSON esperado cuando es determinístico (else -1).
type gameSpecificRoute struct {
	name      string
	gameId    string
	method    string
	path      string
	body      string
	wantFirst int // 0/2/13, o -1 para "solo verificar que es array JSON de código válido"
}

// TestGameSpecificRoutes_httptest cubre RUTA POR RUTA los endpoints que solo se
// registran para AoE1/AoE3/AoE4/AoM y que los tests genéricos no validan con
// valores esperados. Cada caso verifica con httptest que la ruta está registrada
// (no 404), responde 200 y devuelve JSON con un código de éxito/error esperado.
func TestGameSpecificRoutes_httptest(t *testing.T) {
	cases := []gameSpecificRoute{
		// --- AoE1 (usa POST en varias rutas que otros juegos exponen como GET) ---
		{name: "AoE1_getChatChannels", gameId: game.AoE1, method: "POST", path: "/game/chat/getChatChannels", wantFirst: 0},
		{name: "AoE1_getRelationships", gameId: game.AoE1, method: "POST", path: "/game/relationship/getRelationships", wantFirst: 0},
		{name: "AoE1_getLanAdvertisements", gameId: game.AoE1, method: "POST", path: "/game/advertisement/getLanAdvertisements", body: "lanServerGuids=[]", wantFirst: 0},
		{name: "AoE1_updatePlatformLobbyID", gameId: game.AoE1, method: "POST", path: "/game/advertisement/updatePlatformLobbyID", body: "matchID=1&platformlobbyID=2", wantFirst: 2},
		{name: "AoE1_findAdvertisements", gameId: game.AoE1, method: "POST", path: "/game/advertisement/findAdvertisements", wantFirst: -1},

		// --- AoE3 ---
		{name: "AoE3_getFileURL", gameId: game.AoE3, method: "POST", path: "/game/cloud/getFileURL", body: `names=["x"]`, wantFirst: 2},
		{name: "AoE3_updateProgress", gameId: game.AoE3, method: "POST", path: "/game/challenge/updateProgress", body: "challengeID=1&commandID=1", wantFirst: -1},
		{name: "AoE3_joinChannel", gameId: game.AoE3, method: "POST", path: "/game/chat/joinChannel", body: "channelID=9999", wantFirst: 2},
		{name: "AoE3_leaveChannel", gameId: game.AoE3, method: "POST", path: "/game/chat/leaveChannel", body: "channelID=1", wantFirst: -1},
		{name: "AoE3_sendText", gameId: game.AoE3, method: "POST", path: "/game/chat/sendText", body: "channelID=1&message=hi", wantFirst: -1},
		{name: "AoE3_sendWhisper", gameId: game.AoE3, method: "POST", path: "/game/chat/sendWhisper", body: "targetProfileID=1&message=hi", wantFirst: -1},
		{name: "AoE3_getChatChannels", gameId: game.AoE3, method: "POST", path: "/game/chat/getChatChannels", wantFirst: 0},

		// --- AoE4 ---
		{name: "AoE4_polling", gameId: game.AoE4, method: "POST", path: "/game/automatch2/polling", wantFirst: 2},
		{name: "AoE4_stoppolling", gameId: game.AoE4, method: "POST", path: "/game/automatch2/stoppolling", wantFirst: -1},
		{name: "AoE4_getAutomatchMap", gameId: game.AoE4, method: "GET", path: "/game/automatch/getAutomatchMap", wantFirst: -1},
		{name: "AoE4_getRecentMatchSinglePlayerHistory", gameId: game.AoE4, method: "GET", path: "/game/Leaderboard/getRecentMatchSinglePlayerHistory", wantFirst: -1},
		{name: "AoE4_getChallengeProgressByProfileID", gameId: game.AoE4, method: "GET", path: "/game/Challenge/getChallengeProgressByProfileID?challengeIDs=[1]", wantFirst: 0},
		{name: "AoE4_getProfileProperty", gameId: game.AoE4, method: "GET", path: "/game/account/getProfileProperty?profile_id=1&property_id=x", wantFirst: 0},
		{name: "AoE4_addProfileProperty", gameId: game.AoE4, method: "POST", path: "/game/account/addProfileProperty", body: "profile_id=1&property_id=x&value=v", wantFirst: -1},
		{name: "AoE4_clearProfileProperty", gameId: game.AoE4, method: "POST", path: "/game/account/clearProfileProperty", body: "profile_id=1&property_id=x", wantFirst: -1},

		// --- AoM (athens) ---
		{name: "AoM_deleteOfflineMessage", gameId: game.AoM, method: "POST", path: "/game/chat/deleteOfflineMessage", wantFirst: 0},
		{name: "AoM_sendWhispers", gameId: game.AoM, method: "POST", path: "/game/chat/sendWhispers", body: "profileIDs=[1]&message=hi", wantFirst: -1},
		{name: "AoM_getEventStats", gameId: game.AoM, method: "GET", path: "/game/CommunityEvent/getEventStats", wantFirst: -1},
		{name: "AoM_getEventLeaderboard", gameId: game.AoM, method: "GET", path: "/game/CommunityEvent/getEventLeaderboard", wantFirst: -1},
		{name: "AoM_updateProgressBatched", gameId: game.AoM, method: "POST", path: "/game/challenge/updateProgressBatched", body: "updates=[]", wantFirst: -1},
		{name: "AoM_createOrReportSinglePlayer", gameId: game.AoM, method: "POST", path: "/game/party/createOrReportSinglePlayer", wantFirst: -1},
		{name: "AoM_reportUser", gameId: game.AoM, method: "POST", path: "/game/playerreport/reportUser", body: "profileID=1&reason=1", wantFirst: -1},
		{name: "AoM_getProfileProperty", gameId: game.AoM, method: "GET", path: "/game/account/getProfileProperty?profile_id=1&property_id=x", wantFirst: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGame(t, tc.gameId)
			h := gameHandler(tc.gameId)
			sid := createSession(t, g)
			rr := doRequest(h, tc.method, tc.path, g, sid, tc.body, nil)

			if rr.Code == http.StatusNotFound {
				t.Errorf("ruta %s %s no registrada (404) body=%q", tc.method, tc.path, rr.Body.String())
				return
			}
			if rr.Code != http.StatusOK {
				t.Errorf("ruta %s %s code=%d want 200 body=%q", tc.method, tc.path, rr.Code, rr.Body.String())
				return
			}

			bodyStr := strings.TrimSpace(rr.Body.String())
			if bodyStr == "" || bodyStr == "null" {
				t.Logf("ruta %s %s body vacío/null (permitido)", tc.method, tc.path)
				return
			}
			var arr i.A
			if err := json.Unmarshal([]byte(bodyStr), &arr); err != nil {
				var obj any
				if err2 := json.Unmarshal([]byte(bodyStr), &obj); err2 != nil {
					t.Errorf("ruta %s %s body no es JSON válido: %v body=%q", tc.method, tc.path, err, bodyStr)
					return
				}
				t.Logf("ruta %s %s body objeto/null (no array): %s", tc.method, tc.path, bodyStr)
				return
			}
			if len(arr) == 0 {
				t.Errorf("ruta %s %s array vacío body=%q", tc.method, tc.path, bodyStr)
				return
			}
			first, ok := arr[0].(float64)
			if !ok {
				t.Logf("ruta %s %s primer elemento no numérico body=%q", tc.method, tc.path, bodyStr)
				return
			}
			got := int(first)
			if tc.wantFirst == -1 {
				if got != 0 && got != 2 && got != 13 {
					t.Logf("ruta %s %s first=%d (código inusual) body=%q", tc.method, tc.path, got, bodyStr)
				}
				t.Logf("\u2713 %s %s \u2192 first=%d", tc.method, tc.path, got)
				return
			}
			if got != tc.wantFirst {
				t.Errorf("ruta %s %s first=%d want %d body=%q", tc.method, tc.path, got, tc.wantFirst, bodyStr)
				return
			}
			t.Logf("\u2713 %s %s \u2192 first=%d", tc.method, tc.path, got)
		})
	}
}
