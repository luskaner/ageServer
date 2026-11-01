package router

import (
	"bytes"
	"crypto/sha512"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type handlerFirstReader struct {
	started *bool
	body    io.Reader
	t       *testing.T
}

func (r handlerFirstReader) Read(p []byte) (int, error) {
	if !*r.started {
		r.t.Fatal("logger read MatchContent before the handler could extend the deadline")
	}
	return r.body.Read(p)
}

func TestMatchContentLoggingStreamsUpload(t *testing.T) {
	started := false
	payload := bytes.Repeat([]byte("x"), 3<<20)
	req := httptest.NewRequest("POST", "/api/MatchContent/", handlerFirstReader{&started, bytes.NewReader(payload), t})
	h := NewLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started = true
		data, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("payload corrupted: %d %v", len(data), err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestResponseLoggingMemoryBound(t *testing.T) {
	rr := httptest.NewRecorder()
	w := NewResponseWriterWrapper(rr)
	chunk := bytes.Repeat([]byte("x"), 32<<10)
	for n := 0; n < 100; n++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if w.Body.Len() != maxLoggedBodyBytes+1 {
		t.Fatalf("logger retained %d bytes", w.Body.Len())
	}
	sum := sha512.Sum512(rr.Body.Bytes())
	if !bytes.Equal(w.bodyHash.Sum(nil), sum[:]) {
		t.Fatal("response hash was truncated with the preview")
	}
	if rr.Body.Len() != 100*len(chunk) {
		t.Fatal("logger truncated actual download")
	}
}
