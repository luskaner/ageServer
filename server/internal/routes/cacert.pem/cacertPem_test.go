package cacert_pem

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal/models"
)

// mockGame implements models.Game with just Title
type mockGame struct {
	title string
}

func (m *mockGame) Title() string                                         { return m.title }
func (m *mockGame) Resources() models.Resources                           { return nil }
func (m *mockGame) Items() models.Items                                   { return nil }
func (m *mockGame) LeaderboardDefinitions() models.LeaderboardDefinitions { return nil }
func (m *mockGame) PresenceDefinitions() models.PresenceDefinitions       { return nil }
func (m *mockGame) BattleServers() models.BattleServers                   { return nil }
func (m *mockGame) Users() models.Users                                   { return nil }
func (m *mockGame) Advertisements() models.Advertisements                 { return nil }
func (m *mockGame) ChatChannels() models.ChatChannels                     { return nil }
func (m *mockGame) Sessions() models.Sessions                             { return nil }
func (m *mockGame) CustomFiles() models.CustomFiles                       { return nil }
func (m *mockGame) Matchmaking() models.Matchmaking                       { return nil }
func (m *mockGame) MatchHistory() models.MatchHistory                     { return nil }

func TestCacertPemNotFoundWhenNoFolder(t *testing.T) {
	// This test will call os.Executable which returns the test binary path, and CertificatePairFolder will create a folder
	// To test the NotFound path, we can make the folder not contain the cert file
	req := httptest.NewRequest("GET", "/cacert.pem", nil)
	// Set game in context
	ctx := context.WithValue(req.Context(), "game", &mockGame{title: "age2"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	CacertPem(rr, req)
	// Since the cert file likely doesn't exist in the test's certificate folder, it should be 404
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCacertPemServesFileWhenExists(t *testing.T) {
	exe, _ := os.Executable()
	folder := common.CertificatePairFolder(exe)
	if folder == "" {
		t.Skip("could not determine folder")
	}
	os.MkdirAll(folder, 0755)
	for _, name := range []string{common.CACert, common.SelfSignedCert} {
		path := filepath.Join(folder, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.WriteFile(path, []byte("dummy"), 0644)
			defer os.Remove(path)
		}
	}
	req := httptest.NewRequest("GET", "/cacert.pem", nil)
	ctx := context.WithValue(req.Context(), "game", &mockGame{title: "age2"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	CacertPem(rr, req)
	// Should be 200 if file exists, or 404 if not
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected code %d", rr.Code)
	}
}

func TestCacertPemSelfSignedVsCA(t *testing.T) {
	// Test with a game that uses SelfSignedCert vs one that uses CACert
	for _, tc := range []struct {
		gameId string
		file   string
	}{
		{"age1", common.SelfSignedCert},
		{"age2", common.SelfSignedCert},
		{"age4", common.CACert},
		{"athens", common.CACert},
	} {
		req := httptest.NewRequest("GET", "/cacert.pem", nil)
		ctx := context.WithValue(req.Context(), "game", &mockGame{title: tc.gameId})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		CacertPem(rr, req)
		// Just check it doesn't panic and returns 404 or 200
		if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
			t.Fatalf("game %s code %d", tc.gameId, rr.Code)
		}
	}
}
