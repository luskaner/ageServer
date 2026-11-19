package router

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
)

// modsSteamToken builds an old-layout gbe auth ticket, the plaintext kind an
// emulated client sends as `Authorization: Steam1.0`.
func modsSteamToken(steamID uint64) string {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data[0:4], 20)
	binary.LittleEndian.PutUint64(data[12:20], steamID)
	return base64.StdEncoding.EncodeToString(data)
}

func TestProxyModsRequestDecodesSteamTicket(t *testing.T) {
	oldInternet := i.CanUseInternet
	t.Cleanup(func() { i.CanUseInternet = oldInternet })

	request := func(token string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v4/Mods/Games", nil)
		if token != "" {
			req.Header.Set("Authorization", "Steam1.0 "+token)
		}
		return req
	}
	opaque := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa5}, 160))

	cases := []struct {
		name     string
		internet bool
		token    string
		want     bool
	}{
		{"decodable ticket stays local", true, modsSteamToken(76561198000000456), false},
		{"opaque ticket goes upstream", true, opaque, true},
		{"missing token goes upstream", true, "", true},
		{"malformed token goes upstream", true, "not-base64", true},
		{"no internet keeps everything local", false, opaque, false},
		{"no internet keeps decodable tickets local", false, modsSteamToken(76561198000000456), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i.CanUseInternet = tc.internet
			if got := proxyModsRequest("age2", request(tc.token)); got != tc.want {
				t.Fatalf("proxyModsRequest = %v, want %v", got, tc.want)
			}
		})
	}
}
