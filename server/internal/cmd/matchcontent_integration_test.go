package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/server/internal"
)

// matchContentIntegrationConfig is the resolved configuration the runtime would
// publish for [Mods] and [CustomFiles] both enabled, with the storage pointed
// at a temporary folder so the test never writes into resources/.
func matchContentIntegrationConfig(t *testing.T) internal.ModsConfiguration {
	t.Helper()
	dir := t.TempDir()
	return internal.ModsConfiguration{
		DBPath:        filepath.Join(dir, "mods.db"),
		UploadDir:     filepath.Join(dir, "uploads"),
		MaxUploadSize: 1 << 20,
		MatchContent: internal.CustomFilesConfiguration{
			Enabled:      true,
			Folder:       filepath.Join(dir, "match"),
			MaxFileSize:  1 << 20,
			MaxTotalSize: 1 << 30,
			Expiry:       time.Hour,
		},
	}
}

// uploadMatch posts the multipart body the game sends: the raw savegame in the
// fileContent part and the numeric match id in the matchId part.
func uploadMatch(t *testing.T, handler http.Handler, matchId string, savegame string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("fileContent", "match.aoe2mpgame")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(savegame)); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("matchId", matchId); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/MatchContent/", &body)
	request.Header.Set("Content-Type", mw.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestModsServerServesMatchContent pins the transition of the savegame
// endpoints to the embedded library end to end: the same [CustomFiles]
// settings newModsServer hands to it drive a real upload and download,
// through the casings and the trailing slash the game actually sends.
func TestModsServerServesMatchContent(t *testing.T) {
	server, err := newModsServer(matchContentIntegrationConfig(t))
	if err != nil {
		t.Fatalf("newModsServer: %v", err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	handler := server.Handler()

	recorder := uploadMatch(t, handler, "42", "the savegame bytes")
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var uploaded struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("upload response is not the JSON the game parses: %v (%s)", err, recorder.Body.String())
	}
	if uploaded.IsError {
		t.Fatal("the game requires isError = false on a successful upload")
	}

	// The metadata endpoint answers the fileList entry with the absolute
	// download URL the library derives from the request host.
	metadata := httptest.NewRequest(http.MethodGet, "/api/MatchContent/42", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, metadata)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var listed struct {
		Files []struct {
			Filename string `json:"fileName"`
			Path     string `json:"filePath"`
		} `json:"fileList"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("metadata response is not JSON: %v (%s)", err, recorder.Body.String())
	}
	if len(listed.Files) != 1 {
		t.Fatalf("fileList = %+v, want one entry", listed.Files)
	}
	entry := listed.Files[0]
	if entry.Filename != "match.aoe2mpgame" {
		t.Fatalf("fileName = %q", entry.Filename)
	}
	if !strings.HasPrefix(entry.Path, "http://") ||
		!strings.Contains(entry.Path, "/api/MatchContent/42/file/") {
		t.Fatalf("filePath = %q, want an absolute /api/MatchContent/{id}/file/{hash} URL", entry.Path)
	}

	// The payload endpoint streams back exactly what was uploaded, with the
	// digest of the URL guarding the generation.
	parsed, err := url.Parse(entry.Path)
	if err != nil {
		t.Fatalf("filePath %q is not a URL: %v", entry.Path, err)
	}
	download := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, download)
	if recorder.Code != http.StatusOK {
		t.Fatalf("download status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); got != "the savegame bytes" {
		t.Fatalf("download payload = %q, want the uploaded bytes", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

// TestModsServerMatchContentDisabledAnswersNotFound pins the anonymous-tree
// rollback: a library built for the mods API alone answers 404 on the
// savegame endpoints, exactly like the host handlers it replaces.
func TestModsServerMatchContentDisabledAnswersNotFound(t *testing.T) {
	cfg := matchContentIntegrationConfig(t)
	cfg.MatchContent.Enabled = false
	server, err := newModsServer(cfg)
	if err != nil {
		t.Fatalf("newModsServer: %v", err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	handler := server.Handler()

	for _, path := range []string{
		"/api/MatchContent/",
		"/api/MatchContent/123",
		"/api/MatchContent/123/file/deadbeef",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404 from the disabled store", path, recorder.Code)
		}
	}
}
