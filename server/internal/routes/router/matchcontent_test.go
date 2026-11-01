package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// newMatchContentGame builds a game whose CustomFiles instance is enabled and
// rooted at a per-test temp folder. The router under test is built separately
// via gameHandler.
func newMatchContentGame(t *testing.T, gameId string, opts *models.CustomFilesOpts) models.Game {
	t.Helper()
	origAuth := i.Authentication
	i.Authentication = "disabled"
	t.Cleanup(func() { i.Authentication = origAuth })
	origGen := i.GeneratePlatformUserId
	i.GeneratePlatformUserId = false
	t.Cleanup(func() { i.GeneratePlatformUserId = origGen })
	i.InitializeRng(42)

	fake := &fakeResources{}
	fake.Initialize(gameId, nil)
	if opts == nil {
		opts = &models.CustomFilesOpts{}
	}
	opts.Enabled = true
	opts.Folder = t.TempDir()
	if opts.Expiry <= 0 {
		opts.Expiry = time.Hour
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = 1 << 20
	}
	if opts.MaxTotalSize <= 0 {
		opts.MaxTotalSize = 4 << 20
	}
	g := models.CreateMainGame(gameId, &models.CreateMainGameOpts{
		Instances: &models.InstanceOpts{
			Resources:       fake,
			CustomFilesOpts: opts,
		},
	})
	t.Cleanup(g.CustomFiles().Close)
	return g
}

func doRawRequest(handler http.Handler, method, path string, g models.Game, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	ctx := context.WithValue(req.Context(), "game", g)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// buildUploadBody reproduces the multipart capture: fileContent first, then
// matchId.
func buildUploadBody(t *testing.T, filename string, content []byte, matchId string, withFile, withMatch bool) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if withFile {
		fw, err := mw.CreateFormFile("fileContent", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if withMatch {
		if err := mw.WriteField("matchId", matchId); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

type matchManifest struct {
	IsError bool `json:"isError"`
	Files   []struct {
		Name string `json:"fileName"`
		Size int    `json:"fileSize"`
		Path string `json:"filePath"`
		CRC  string `json:"fileCRC"`
	} `json:"fileList"`
}

func downloadMatchFile(t *testing.T, h http.Handler, g models.Game, path string) *httptest.ResponseRecorder {
	t.Helper()
	metadata := doRawRequest(h, "GET", path, g, nil, nil)
	if metadata.Code != http.StatusOK {
		t.Fatalf("metadata status=%d body=%q", metadata.Code, metadata.Body.String())
	}
	var manifest matchManifest
	if err := json.Unmarshal(metadata.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.IsError || len(manifest.Files) != 1 {
		t.Fatalf("invalid manifest: %+v", manifest)
	}
	f := manifest.Files[0]
	u, err := url.Parse(f.Path)
	if err != nil || u.Host != "example.com" || u.Scheme != "http" {
		t.Fatalf("invalid file URL: %q (%v)", f.Path, err)
	}
	result := doRawRequest(h, "GET", u.RequestURI(), g, nil, nil)
	if result.Code != http.StatusOK {
		t.Fatalf("payload status=%d body=%q", result.Code, result.Body.String())
	}
	if f.Size != result.Body.Len() || f.CRC != fmt.Sprintf("%08x", crc32.ChecksumIEEE(result.Body.Bytes())) {
		t.Fatalf("metadata differs from payload: %+v", f)
	}
	return result
}

func TestMatchContentUploadDownload_httptest(t *testing.T) {
	for _, gid := range []string{game.AoE1, game.AoE2, game.AoE3, game.AoE4, game.AoM} {
		t.Run(gid, func(t *testing.T) {
			g := newMatchContentGame(t, gid, nil)
			h := gameHandler(gid)
			content := []byte("AOE2 multiplayer savegame bytes")

			body, contentType := buildUploadBody(t, "TEST2.aoe2mpgame", content, "234876462", true, true)
			rr := doRawRequest(h, "POST", "/api/MatchContent/", g, body, map[string]string{"Content-Type": contentType})
			if rr.Code != http.StatusOK {
				t.Fatalf("upload code = %d want 200 body=%q", rr.Code, rr.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil || response["isError"] != false {
				t.Fatalf("upload response must contain boolean isError:false: %s (%v)", rr.Body.String(), err)
			}
			if g.CustomFiles().Len() != 1 {
				t.Fatalf("expected one stored file, got %d", g.CustomFiles().Len())
			}

			// Download must work without a session (the game does not send one).
			dl := downloadMatchFile(t, h, g, "/api/MatchContent/234876462")
			if dl.Code != http.StatusOK {
				t.Fatalf("download code = %d want 200 body=%q", dl.Code, dl.Body.String())
			}
			if !bytes.Equal(dl.Body.Bytes(), content) {
				t.Fatalf("downloaded %q want %q", dl.Body.Bytes(), content)
			}
			if got := dl.Header().Get("Content-Length"); got != strconv.Itoa(len(content)) {
				t.Fatalf("Content-Length = %q want %d", got, len(content))
			}
			if got := dl.Header().Get("Content-Type"); got != "application/octet-stream" {
				t.Fatalf("Content-Type = %q", got)
			}
			if dl.Header().Get("Content-MD5") == "" || dl.Header().Get("ETag") == "" {
				t.Fatal("download is missing MD5/ETag headers")
			}

			// Unknown id => 404.
			missing := doRawRequest(h, "GET", "/api/MatchContent/999", g, nil, nil)
			if missing.Code != http.StatusNotFound {
				t.Fatalf("missing id code = %d want 404", missing.Code)
			}
		})
	}
}

func TestMatchContentUploadInvalid_httptest(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, nil)
	h := gameHandler(game.AoE2)

	cases := []struct {
		name      string
		filename  string
		content   []byte
		matchId   string
		withFile  bool
		withMatch bool
	}{
		{"no_match_id", "TEST2.aoe2mpgame", []byte("data"), "", true, false},
		{"zero_match_id", "TEST2.aoe2mpgame", []byte("data"), "0", true, true},
		{"invalid_match_id", "TEST2.aoe2mpgame", []byte("data"), "abc", true, true},
		{"no_file", "", nil, "55", false, true},
		{"empty_file", "TEST2.aoe2mpgame", nil, "55", true, true},
		{"unknown_extension", "save.zip", []byte("data"), "55", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, contentType := buildUploadBody(t, tc.filename, tc.content, tc.matchId, tc.withFile, tc.withMatch)
			rr := doRawRequest(h, "POST", "/api/MatchContent/", g, body, map[string]string{"Content-Type": contentType})
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("code = %d want 400 body=%q", rr.Code, rr.Body.String())
			}
			if g.CustomFiles().Len() != 0 {
				t.Fatalf("rejected upload was stored (%d entries)", g.CustomFiles().Len())
			}
		})
	}
}

func TestMatchContentUploadTooLarge_httptest(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, &models.CustomFilesOpts{MaxFileSize: 16, MaxTotalSize: 1024})
	h := gameHandler(game.AoE2)
	content := bytes.Repeat([]byte("A"), 32)
	body, contentType := buildUploadBody(t, "TEST2.aoe2mpgame", content, "1", true, true)
	rr := doRawRequest(h, "POST", "/api/MatchContent/", g, body, map[string]string{"Content-Type": contentType})
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d want 413 body=%q", rr.Code, rr.Body.String())
	}
	if g.CustomFiles().Len() != 0 {
		t.Fatal("oversized upload was stored")
	}
}

// TestMatchContentUploadOverGlobalCap_httptest is the regression guard for the
// body-cap exemption: a savegame bigger than the server-wide 1 MiB cap has to
// reach the handler and be judged by CustomFiles.MaxFileSize, and being turned
// away by the global reader must still be reported as 413 rather than 500.
func TestMatchContentUploadOverGlobalCap_httptest(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, &models.CustomFilesOpts{
		MaxFileSize:  8 << 20,
		MaxTotalSize: 16 << 20,
	})
	h := MaxBodySizeMiddleware(1<<20, gameHandler(game.AoE2))

	// Bigger than the 1 MiB global cap, smaller than MaxFileSize: accepted.
	content := bytes.Repeat([]byte{0x11}, 3<<20)
	body, contentType := buildUploadBody(t, "TEST2.aoe2mpgame", content, "9", true, true)
	rr := doRawRequest(h, "POST", "/api/MatchContent/", g, body, map[string]string{"Content-Type": contentType})
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d want 200 body=%q", rr.Code, rr.Body.String())
	}
	dl := downloadMatchFile(t, h, g, "/api/MatchContent/9")
	if dl.Code != http.StatusOK || !bytes.Equal(dl.Body.Bytes(), content) {
		t.Fatalf("download code=%d len=%d want %d", dl.Code, dl.Body.Len(), len(content))
	}

	// Bigger than MaxFileSize: rejected, and as 413 whatever tripped first.
	tooBig := bytes.Repeat([]byte{0x22}, 9<<20)
	body2, contentType2 := buildUploadBody(t, "TEST2.aoe2mpgame", tooBig, "10", true, true)
	rr2 := doRawRequest(h, "POST", "/api/MatchContent/", g, body2, map[string]string{"Content-Type": contentType2})
	if rr2.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d want 413 body=%q", rr2.Code, rr2.Body.String())
	}
	if _, _, ok := g.CustomFiles().Open(10); ok {
		t.Fatal("rejected oversized upload was stored")
	}
}

func TestMatchContentOverwrite_httptest(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, nil)
	h := gameHandler(game.AoE2)
	first := []byte("first version")
	second := []byte("second version longer")
	for _, payload := range [][]byte{first, second} {
		body, contentType := buildUploadBody(t, "TEST2.aoe2mpgame", payload, "42", true, true)
		rr := doRawRequest(h, "POST", "/api/MatchContent/", g, body, map[string]string{"Content-Type": contentType})
		if rr.Code != http.StatusOK {
			t.Fatalf("upload code = %d", rr.Code)
		}
	}
	if g.CustomFiles().Len() != 1 {
		t.Fatalf("len = %d want 1", g.CustomFiles().Len())
	}
	dl := downloadMatchFile(t, h, g, "/api/MatchContent/42")
	if !bytes.Equal(dl.Body.Bytes(), second) {
		t.Fatalf("content = %q want %q", dl.Body.Bytes(), second)
	}
}

// TestMatchContentApiAgeOfEmpiresRoute checks the routes are registered on the
// api.ageofempires.com mux (so they win over the reverse proxy) and that the
// handler is reached.
func TestMatchContentApiAgeOfEmpiresRoute_httptest(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, nil)
	h := NewApiAgeOfEmpires().InitializeRoutes(game.AoE2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request for %s reached the upstream fallback", r.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
	}))

	body, contentType := buildUploadBody(t, "TEST2.aoe2mpgame", []byte("data"), "77", true, true)
	req := httptest.NewRequest("POST", "/api/MatchContent/", body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(context.WithValue(req.Context(), "game", g))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound || rr.Code == http.StatusBadGateway {
		t.Fatalf("route not handled locally: code=%d", rr.Code)
	}

	dl := httptest.NewRequest("GET", "/api/MatchContent/77", nil)
	dl = dl.WithContext(context.WithValue(dl.Context(), "game", g))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, dl)
	if rr2.Code != http.StatusOK {
		t.Fatalf("download via api mux code = %d", rr2.Code)
	}
}

// This exercises real TLS sockets and the same two-stage flow used by the
// game, rather than accepting an arbitrary 200 response as success.
func TestMatchContentTLSContract(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, nil)
	for _, apiHost := range []bool{false, true} {
		t.Run(strconv.FormatBool(apiHost), func(t *testing.T) {
			var h http.Handler = gameHandler(game.AoE2)
			if apiHost {
				h = NewApiAgeOfEmpires().InitializeRoutes(game.AoE2, http.NotFoundHandler())
			}
			h = MaxBodySizeMiddleware(1<<20, NewLoggingMiddleware(h))
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "game", g)))
			}))
			defer ts.Close()
			client := ts.Client()
			content := []byte("123456789") // Independent, standard CRC-32/IEEE test vector.
			body, ct := buildUploadBody(t, "Сохранение (Alex).aoe2mpgame", content, "123", true, true)
			response, err := client.Post(ts.URL+"/api/MatchContent/", ct, body)
			if err != nil {
				t.Fatal(err)
			}
			var upload map[string]any
			err = json.NewDecoder(response.Body).Decode(&upload)
			response.Body.Close()
			if response.StatusCode != 200 || err != nil || upload["isError"] != false {
				t.Fatalf("upload: %d %+v %v", response.StatusCode, upload, err)
			}
			response, err = client.Get(ts.URL + "/api/MatchContent/123")
			if err != nil {
				t.Fatal(err)
			}
			var metadata matchManifest
			err = json.NewDecoder(response.Body).Decode(&metadata)
			response.Body.Close()
			if response.StatusCode != 200 || err != nil || metadata.IsError || len(metadata.Files) != 1 {
				t.Fatalf("metadata: %d %+v %v", response.StatusCode, metadata, err)
			}
			f := metadata.Files[0]
			if f.Name != "Сохранение (Alex).aoe2mpgame" || f.Size != 9 || f.CRC != "cbf43926" {
				t.Fatalf("metadata: %+v", f)
			}
			if !strings.HasPrefix(f.Path, ts.URL+"/api/MatchContent/123/file/") {
				t.Fatalf("URL must preserve HTTPS and port: %q", f.Path)
			}
			response, err = client.Get(f.Path)
			if err != nil {
				t.Fatal(err)
			}
			downloaded, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != 200 || !bytes.Equal(downloaded, content) {
				t.Fatalf("payload: %d %q %v", response.StatusCode, downloaded, err)
			}
			if response.ContentLength != int64(len(content)) {
				t.Fatalf("length: %d", response.ContentLength)
			}
			if response.Header.Get("Content-MD5") != "JfnnlDI7RTiF9RgfG2JNCw==" {
				t.Fatalf("MD5: %s", response.Header.Get("Content-MD5"))
			}
			// Metadata from before an overwrite must never fetch the new bytes.
			body, ct = buildUploadBody(t, "next.aoe2mpgame", []byte("replacement"), "123", true, true)
			response, err = client.Post(ts.URL+"/api/MatchContent/", ct, body)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			response, err = client.Get(f.Path)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("stale URL: %d", response.StatusCode)
			}
		})
	}
}

func TestMatchContentDownloadInvalid(t *testing.T) {
	g := newMatchContentGame(t, game.AoE2, nil)
	h := gameHandler(game.AoE2)
	for _, path := range []string{"0", "-1", "2147483648", "abc", "123/file/../../outside"} {
		rr := doRawRequest(h, "GET", "/api/MatchContent/"+path, g, nil, nil)
		if rr.Code == http.StatusOK {
			t.Fatalf("invalid path accepted: %s", path)
		}
	}
}
