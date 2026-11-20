package models

import "errors"

// ErrFileTooLarge is returned by the upload stores when a stream exceeds the
// configured per-file limit.
var ErrFileTooLarge = errors.New("upload exceeds the maximum size")

// limitedWriter fails as soon as a write would push the total past limit. A
// non-positive limit means no per-file limit, not "reject everything".
type limitedWriter struct {
	limit int64
	n     int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.limit > 0 && lw.n+int64(len(p)) > lw.limit {
		return 0, ErrFileTooLarge
	}
	lw.n += int64(len(p))
	return len(p), nil
}
