package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func uploadPath(key string) string {
	return "/cloudfiles/" + key
}

func testCredential(t *testing.T, g models.Game, key string) string {
	t.Helper()
	store := g.Resources().CloudFiles()
	id, _, ok := store.Credential(key)
	if !ok {
		t.Fatal("Credential failed on the test store")
	}
	return id
}

func TestCloudfilesReplayPutGetRoundTrip(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := gameHandler(game.AoE2)
	key := "replays/123/0/abcd.gz"
	sig := testCredential(t, g, key)
	path := uploadPath(key) + "?sig=" + url.QueryEscape(sig)

	rr := doRequest(h, "PUT", path, g, "", "replay-bytes", nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("PUT got %d body=%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("ETag") == "" || rr.Header().Get("x-ms-request-id") == "" {
		t.Fatalf("PUT headers missing: %v", rr.Header())
	}

	rr = doRequest(h, "GET", path, g, "", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET got %d body=%q", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "replay-bytes" {
		t.Fatalf("GET body = %q", rr.Body.String())
	}
	if got := rr.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestCloudfilesRejectsBadSignatures(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	h := gameHandler(game.AoE2)
	key := "replays/1/0/x.gz"
	otherSignature := testCredential(t, g, "replays/2/0/other.gz")

	rr := doRequest(h, "PUT", uploadPath(key), g, "", "bytes", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("PUT without sig got %d, want 401", rr.Code)
	}
	rr = doRequest(h, "PUT", uploadPath(key)+"?sig="+url.QueryEscape(otherSignature), g, "", "bytes", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("PUT with another key's sig got %d, want 403", rr.Code)
	}
}

func TestCloudfilesUploadTooLarge(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	g.Resources().CloudFiles().SetReplayOpts(models.CloudFilesReplayOpts{
		Enabled:      true,
		MaxFileSize:  4,
		MaxTotalSize: 1 << 20,
		Expiry:       -1,
	})
	h := gameHandler(game.AoE2)
	key := "replays/5/0/big.gz"
	sig := testCredential(t, g, key)
	rr := doRequest(h, "PUT", uploadPath(key)+"?sig="+url.QueryEscape(sig), g, "", "way too large", nil)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("PUT got %d, want 413 (body %q)", rr.Code, rr.Body.String())
	}
	if _, ok := g.Resources().CloudFiles().Get(key); ok {
		t.Fatal("a rejected upload was published")
	}
}

func TestCloudfilesGetTempCredentialsDoesNotPanicWithoutAssets(t *testing.T) {
	for _, gameID := range []string{game.AoE4, game.AoM} {
		t.Run(gameID, func(t *testing.T) {
			g := newTestGame(t, gameID)
			h := gameHandler(gameID)
			sid := createSession(t, g)
			rr := doRequest(h, "GET", "/game/cloud/getTempCredentials?key=/cloudfiles/replays/1/0/x.gz", g, sid, "", nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("getTempCredentials got %d", rr.Code)
			}
			var response i.A
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response) < 4 || response[0] != float64(2) {
				t.Fatalf("getTempCredentials = %#v, want code 2 for an unknown key", response)
			}
		})
	}
}

func TestGetFileURLResolvesBuiltinPrefix(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	store := g.Resources().CloudFiles()
	store.Value["builtin/_data.txt"] = models.CloudfilesIndex{
		Id:       1,
		Key:      "game/abc123",
		Type:     "text/plain",
		Version:  "2019-02-02",
		Length:   5,
		Checksum: "x",
	}
	h := gameHandler(game.AoE2)
	sid := createSession(t, g)
	rr := doRequest(h, "GET", "/game/cloud/getFileURL?names=%5B%22_data.txt%22%5D", g, sid, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("getFileURL got %d", rr.Code)
	}
	var response i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response[0] != float64(0) {
		t.Fatalf("getFileURL code = %v, body %s", response[0], rr.Body.String())
	}
	encoded, _ := json.Marshal(response[1])
	if !strings.Contains(string(encoded), "game/abc123") {
		t.Fatalf("getFileURL did not resolve builtin/_data.txt: %s", encoded)
	}
}

func TestFinalizeReplayUploadDeletesOnFailure(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	sid := createSession(t, g)
	store := g.Resources().CloudFiles()
	key := "replays/900/0/a.gz"
	if _, err := store.Put(key, func(w io.Writer) error {
		_, err := w.Write([]byte("x"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	form := fmt.Sprintf("match_id=900&url=http://host/cloudfiles/%s&size=1&finalizeResult=0&isSinglePlayer=0", key)
	rr := doRequest(h, "POST", "/game/party/finalizeReplayUpload", g, sid, form, nil)
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != "[0]" {
		t.Fatalf("finalize got %d %q", rr.Code, rr.Body.String())
	}
	if _, ok := store.Get(key); ok {
		t.Fatal("finalizeResult != 1 did not discard the replay")
	}

	rr = doRequest(h, "POST", "/game/party/finalizeReplayUpload", g, sid, "match_id=900&url=not-a-cloud-url&finalizeResult=1", nil)
	if strings.TrimSpace(rr.Body.String()) != "[0]" {
		t.Fatalf("finalize without /cloudfiles/ got %q", rr.Body.String())
	}
}

// TestGetTempCredentialsDoesNotIssueForUnknownKey guards the fix in the
// handler: issuing the credential before validating the key left a live
// session for credentialsExpiry on every call with a key that is not stored.
func TestGetTempCredentialsDoesNotIssueForUnknownKey(t *testing.T) {
	g := newTestGame(t, game.AoE4)
	h := gameHandler(game.AoE4)
	sid := createSession(t, g)
	store := g.Resources().CloudFiles()

	count := func() int {
		total := 0
		for range store.Credentials.Values() {
			total++
		}
		return total
	}
	before := count()
	for range 3 {
		rr := doRequest(h, "GET",
			"/game/cloud/getTempCredentials?key=/cloudfiles/replays/9/0/missing.gz",
			g, sid, "", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("getTempCredentials got %d", rr.Code)
		}
		var response i.A
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response) < 4 || response[0] != float64(2) {
			t.Fatalf("getTempCredentials = %#v, want code 2 for an unknown key", response)
		}
	}
	if after := count(); after != before {
		t.Fatalf("credentials grew from %d to %d for an unknown key", before, after)
	}
}

// TestCloudfilesReplayUploadOverGlobalBodyCap guards the fix in
// raisedBodySizePrefixes. /cloudfiles/ used to sit behind the global 1 MiB body
// cap, and the logging middleware buffered the body with io.ReadAll discarding
// the MaxBytesError, so the handler saw a clean EOF and published a truncated
// replay with a 201.
func TestCloudfilesReplayUploadOverGlobalBodyCap(t *testing.T) {
	g := newTestGame(t, game.AoE2)
	g.Resources().CloudFiles().SetReplayOpts(models.CloudFilesReplayOpts{
		Enabled:      true,
		MaxFileSize:  32 << 20,
		MaxTotalSize: 1 << 30,
		Expiry:       -1,
	})
	h := MaxBodySizeMiddleware(1<<20, NewLoggingMiddleware(gameHandler(game.AoE2)))
	key := "replays/4242/0/big.gz"
	sig := testCredential(t, g, key)
	body := strings.Repeat("x", 1116937)
	rr := doRequest(h, "PUT", uploadPath(key)+"?sig="+url.QueryEscape(sig), g, "", body, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("PUT got %d, want 201 (body %q)", rr.Code, rr.Body.String())
	}
	entry, ok := g.Resources().CloudFiles().Get(key)
	if !ok {
		t.Fatal("upload was not published")
	}
	if entry.Length != len(body) {
		t.Fatalf("stored length = %d, want %d", entry.Length, len(body))
	}
}

// TestCloudfilesServesBuiltinAssetByBlobKey guards the download path after the
// builtin/ reindexing. Value is keyed by the storage path (builtin/_data.txt)
// but the client asks for the blob key from cloudfilesIndex.json
// (game/<sha256>), so the handler has to resolve one to the other.
func TestCloudfilesServesBuiltinAssetByBlobKey(t *testing.T) {
	cloudFolder := filepath.Join(t.TempDir(), "cloud")
	builtinDir := filepath.Join(cloudFolder, "builtin")
	if err := os.MkdirAll(builtinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("hello builtin")
	if err := os.WriteFile(filepath.Join(builtinDir, "_data.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	g := newTestGameWithCloud(t, game.AoE2, cloudFolder)
	store := g.Resources().CloudFiles()
	store.Value["builtin/_data.txt"] = models.CloudfilesIndex{
		Key:     "game/abc123",
		Type:    "text/plain",
		Version: "2019-02-02",
		Length:  len(content),
	}
	h := gameHandler(game.AoE2)
	sig := testCredential(t, g, "game/abc123")
	rr := doRequest(h, "GET", "/cloudfiles/game/abc123?sig="+url.QueryEscape(sig), g, "", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET got %d, want 200 (body %q)", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != string(content) {
		t.Fatalf("body = %q, want %q", rr.Body.String(), content)
	}
	if got := rr.Header().Get("x-ms-meta-filename"); got != "_data.txt" {
		t.Fatalf("x-ms-meta-filename = %q, want _data.txt", got)
	}
}
