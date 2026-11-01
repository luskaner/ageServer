package cloudfiles

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/server/internal"
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
	cloudfiles := models.G(r).Resources().CloudFiles()
	info, exists := (*cloudfiles.Credentials).Get(r.URL.Query().Get("sig"))

	if !exists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filename, file, ok := cloudfiles.GetByKey(key)
	if ok {
		if file.Key != *info.Data() {
			http.Error(w, "Incorrect signature", http.StatusForbidden)
			return
		}
		data, err := cloudfiles.ReadFile(filename)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		lengthStr := strconv.Itoa(file.Length)
		w.Header().Set("Content-Length", lengthStr)
		w.Header().Set("Content-Type", file.Type)
		w.Header().Set("Content-MD5", file.Checksum)
		w.Header().Set("Last-Modified", file.Created)
		w.Header().Set("Accept-Range", "bytes")
		w.Header().Set("ETag", file.ETag)
		w.Header().Set("Server", "Windows-Azure-Blob/1.0 Microsoft-HTTPAPI/2.0")
		w.Header().Set("x-ms-request-id", generateRequestId())
		w.Header().Set("x-ms-version", file.Version)
		// TODO: Review
		if models.G(r).Title() != game.AoE3 && models.G(r).Title() != game.AoM {
			w.Header().Set("x-ms-meta-filename", filename)
			w.Header().Set("x-ms-meta-ContentLength", lengthStr)
		}
		w.Header().Set("x-ms-creation-time", file.Created)
		w.Header().Set("x-ms-lease-status", "unlocked")
		w.Header().Set("x-ms-lease-state", "available")
		w.Header().Set("x-ms-blob-type", "BlockBlob")
		w.Header().Set("x-ms-server-encrypted", "true")
		w.Header().Set("Date", time.Now().Format(time.RFC1123))
		w.Header().Set("Content-Type", file.Type)
		// Serve the indexed payload through the standard file-serving helper:
		// the bytes come from disk, so they must not be treated as a verbatim
		// copy of the request into the response.
		http.ServeContent(w, r, filename, time.Time{}, bytes.NewReader(data))
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}
