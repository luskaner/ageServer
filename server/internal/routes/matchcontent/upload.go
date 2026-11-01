package matchcontent

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

// multipartSlack is added to the per-file limit before it is applied to the raw
// request body, so the multipart framing (boundaries and part headers) never
// trips the reader before the counting writer enforces the exact file size.
const multipartSlack = 1 << 20

const (
	// minUploadRate is the slowest sustained upload rate a client is given
	// credit for; anything slower has its deadline recomputed upwards.
	minUploadRate = 64 << 10
	// minUploadDeadline covers a small file on a slow link without depending on
	// MaxFileSize, and maxUploadDeadline keeps a stalled client bounded even
	// when the cap is set to hundreds of megabytes.
	minUploadDeadline = 30 * time.Second
	maxUploadDeadline = 5 * time.Minute
)

// extendReadDeadline lifts the read deadline for this request. The server-wide
// ReadTimeout is sized for the small form posts the game normally makes, and a
// savegame would be cut off well before it finished arriving. The deadline is
// still bounded by MaxFileSize instead of being cleared outright, so a client
// that opens the connection and then stalls cannot hold it forever.
func extendReadDeadline(w http.ResponseWriter, maxFileSize int64) {
	deadline := minUploadDeadline
	if maxFileSize > 0 {
		if needed := time.Duration(maxFileSize/minUploadRate) * time.Second; needed > deadline {
			deadline = needed
		}
	}
	if deadline > maxUploadDeadline {
		deadline = maxUploadDeadline
	}
	// A recorder has no connection to set a deadline on; the error is expected
	// in tests and irrelevant in production.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(deadline))
}

// Upload implements POST /api/MatchContent/. The multipart body carries the
// raw savegame in the "fileContent" part and the advertisement id in the
// "matchId" part. The payload never passes through memory: it is streamed to a
// provisional file and only renamed into place once the matchId is known.
//
// AoE2 DE parses the response as JSON and requires the boolean isError field.
func Upload(w http.ResponseWriter, r *http.Request) {
	files := models.G(r).CustomFiles()
	if files == nil || !files.Enabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	maxFileSize := files.MaxFileSize()
	extendReadDeadline(w, maxFileSize)
	if maxFileSize > 0 && maxFileSize < math.MaxInt64-multipartSlack {
		r.Body = http.MaxBytesReader(w, r.Body, maxFileSize+multipartSlack)
	}
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	var stage *models.CustomFileStage
	var matchId int32
	var haveMatchId bool
	var filename string
	var contentType string
	defer func() { files.Discard(stage) }()
	for {
		part, partErr := mr.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			if tooLarge(partErr) {
				http.Error(w, "Content Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		switch part.FormName() {
		case "fileContent":
			if stage != nil {
				_ = part.Close()
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			filename = part.FileName()
			contentType = part.Header.Get("Content-Type")
			stage, err = files.Stage(func(dst io.Writer) error {
				_, copyErr := io.Copy(dst, part)
				return copyErr
			})
			if err != nil {
				if tooLarge(err) {
					http.Error(w, "Content Too Large", http.StatusRequestEntityTooLarge)
					return
				}
				logger.Warn("Failed to stage custom file upload: %s", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		case "matchId":
			value, readErr := io.ReadAll(io.LimitReader(part, 64))
			if readErr != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			id, parseErr := strconv.ParseInt(strings.TrimSpace(string(value)), 10, 32)
			if parseErr != nil || id <= 0 {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			matchId = int32(id)
			haveMatchId = true
		default:
			_, _ = io.Copy(io.Discard, part)
		}
		_ = part.Close()
	}
	if stage == nil || !haveMatchId || stage.Length() <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if !models.KnownCustomFileExtension(filename) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	cf, err := files.Adopt(stage, matchId, filename, contentType)
	if err != nil {
		logger.Warn("Failed to store custom file %d: %s", matchId, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Info("Stored custom file %d (%s, %d bytes)", cf.Id, cf.Filename, cf.Length)
	respondUpload(w)
}

// tooLarge reports whether err is the upload being rejected on size. Either the
// counting writer in the store or the raw body reader can be the one to trip,
// depending on how much multipart framing reaches the reader before the file
// does, and both mean the same thing to the client.
func tooLarge(err error) bool {
	if errors.Is(err, models.ErrFileTooLarge) {
		return true
	}
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func respondUpload(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(struct {
		IsError bool `json:"isError"`
	}{})
}
