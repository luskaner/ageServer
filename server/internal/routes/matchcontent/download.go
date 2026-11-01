package matchcontent

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

type fileMetadata struct {
	Filename string `json:"fileName"`
	Size     int    `json:"fileSize"`
	Path     string `json:"filePath"`
	CRC      string `json:"fileCRC"`
}

// Download implements GET /api/MatchContent/{matchId}. AoE2 DE expects JSON
// metadata here, then fetches the payload separately from filePath.
func Download(w http.ResponseWriter, r *http.Request) {
	cf, rc, ok := openFile(w, r)
	if !ok {
		return
	}
	defer func() { _ = rc.Close() }()
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fileURL := url.URL{
		Scheme: scheme,
		Host:   r.Host,
		Path:   "/api/MatchContent/" + strconv.FormatInt(int64(cf.Id), 10) + "/file/" + cf.MD5,
	}
	name := cf.OriginalFilename
	if name == "" {
		name = cf.Filename
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		IsError bool           `json:"isError"`
		Files   []fileMetadata `json:"fileList"`
	}{Files: []fileMetadata{{Filename: name, Size: cf.Length, Path: fileURL.String(), CRC: cf.CRC32}}})
}

// DownloadFile serves only the indexed payload. The digest in the URL prevents
// stale metadata from downloading a replacement uploaded under the same id.
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	cf, rc, ok := openFile(w, r)
	if !ok {
		return
	}
	defer func() { _ = rc.Close() }()
	if r.PathValue("hash") != cf.MD5 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	respondDownload(w, cf, rc)
}

func openFile(w http.ResponseWriter, r *http.Request) (*models.CustomFile, io.ReadCloser, bool) {
	files := models.G(r).CustomFiles()
	if files == nil || !files.Enabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return nil, nil, false
	}
	id, err := strconv.ParseInt(r.PathValue("matchId"), 10, 32)
	if err != nil || id <= 0 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return nil, nil, false
	}
	cf, rc, ok := files.Open(int32(id))
	if !ok {
		logger.Warn("Custom file %d requested but not found", id)
		http.Error(w, "Not Found", http.StatusNotFound)
		return nil, nil, false
	}
	return cf, rc, true
}

// respondDownload streams the payload without encoding it as JSON or base64.
func respondDownload(w http.ResponseWriter, cf *models.CustomFile, rc io.ReadCloser) {
	w.Header().Set("Content-Type", cf.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(cf.Length))
	w.Header().Set("Content-Disposition", `attachment; filename="`+cf.Filename+`"`)
	w.Header().Set("Last-Modified", cf.Created.UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", `"`+cf.MD5+`"`)
	if raw, err := hex.DecodeString(cf.MD5); err == nil {
		w.Header().Set("Content-MD5", base64.StdEncoding.EncodeToString(raw))
	}
	_, _ = io.Copy(w, rc)
}
