package cloudfiles

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

func generateRequestId() string {
	var u [16]byte
	internal.WithRng(func(rand *internal.RandReader) {
		for i := range 10 {
			u[i] = byte(rand.UintN(256))
		}
	})
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	copy(u[10:], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u[0:4],
		u[4:6],
		u[6:8],
		u[8:10],
		u[10:16],
	)
}

func Cloudfiles(w http.ResponseWriter, r *http.Request) {
	key := strings.Join(strings.Split(r.URL.Path, "/")[2:], "/")
	store := models.G(r).Resources().CloudFiles()
	if store == nil || !store.Enabled() || store.Credentials == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	info, exists := store.Credentials.Get(r.URL.Query().Get("sig"))
	if !exists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if *info.Data() != key {
		http.Error(w, "Incorrect signature", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPut, http.MethodPost:
		upload(w, r, store, key)
	default:
		download(w, r, store, key)
	}
}

func upload(w http.ResponseWriter, r *http.Request, store *models.CloudFiles, key string) {
	// The server-wide ReadTimeout is sized for the small form posts the game
	// normally makes; a replay would be cut off mid-transfer. The store's own
	// MaxFileSize still bounds the deadline.
	internal.ExtendReadDeadline(w, store.MaxFileSize())
	entry, err := store.Put(key, func(dst io.Writer) error {
		_, copyErr := io.Copy(dst, r.Body)
		return copyErr
	})
	switch {
	case errors.Is(err, models.ErrFileTooLarge):
		http.Error(w, "Content Too Large", http.StatusRequestEntityTooLarge)
	case errors.Is(err, models.ErrNotReplay), errors.Is(err, models.ErrReplaysDisabled):
		http.Error(w, "Forbidden", http.StatusForbidden)
	case err != nil:
		logger.Warn("Failed to store cloud file %s: %s", key, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	default:
		w.Header().Set("ETag", entry.ETag)
		w.Header().Set("x-ms-request-id", generateRequestId())
		w.Header().Set("x-ms-version", entry.Version)
		w.WriteHeader(http.StatusCreated)
	}
}

func download(w http.ResponseWriter, r *http.Request, store *models.CloudFiles, key string) {
	// The URL key is the blob key. For replays it is also the storage path, but
	// a builtin asset is served as game/<sha256> and stored as builtin/<name>,
	// so it has to be resolved before opening.
	storagePath, _, ok := store.GetByKey(key)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	file, reader, ok := store.Open(storagePath)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer reader.Close()
	seeker, ok := reader.(io.ReadSeeker)
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lengthStr := strconv.Itoa(file.Length)
	w.Header().Set("Content-Length", lengthStr)
	w.Header().Set("Content-Type", file.Type)
	if file.Checksum != "" {
		w.Header().Set("Content-MD5", file.Checksum)
	}
	w.Header().Set("Last-Modified", file.Created)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", file.ETag)
	w.Header().Set("Server", "Windows-Azure-Blob/1.0 Microsoft-HTTPAPI/2.0")
	w.Header().Set("x-ms-request-id", generateRequestId())
	w.Header().Set("x-ms-version", file.Version)
	// TODO: Review
	if models.G(r).Title() != game.AoE3 && models.G(r).Title() != game.AoM {
		w.Header().Set("x-ms-meta-filename", path.Base(storagePath))
		w.Header().Set("x-ms-meta-ContentLength", lengthStr)
	}
	w.Header().Set("x-ms-creation-time", file.Created)
	w.Header().Set("x-ms-lease-status", "unlocked")
	w.Header().Set("x-ms-lease-state", "available")
	w.Header().Set("x-ms-blob-type", "BlockBlob")
	w.Header().Set("x-ms-server-encrypted", "true")
	w.Header().Set("Date", time.Now().Format(time.RFC1123))
	http.ServeContent(w, r, path.Base(storagePath), time.Time{}, seeker)
}
