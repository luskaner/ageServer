package router

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common/logger/serverCommunication"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/request"
	"github.com/luskaner/ageServer/server/internal/logger"
)

type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
	bodyHash   hash.Hash
}

func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           new(bytes.Buffer),
		bodyHash:       sha512.New(),
	}
}

func (w *ResponseWriterWrapper) WriteHeader(code int) {
	if w.StatusCode == http.StatusOK {
		w.StatusCode = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *ResponseWriterWrapper) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	_, _ = w.bodyHash.Write(b[:n])
	_, _ = boundedLogWriter{w.Body}.Write(b[:n])
	return n, err
}

func (w *ResponseWriterWrapper) Header() http.Header {
	return w.ResponseWriter.Header()
}

// Unwrap lets http.NewResponseController reach the underlying connection, which
// handlers need to adjust per-request deadlines (an upload raises the read
// deadline beyond the server-wide one). Without it the chain stops here.
func (w *ResponseWriterWrapper) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}

// maxLoggedBodyBytes caps what goes into the communication log. A savegame
// transfer may be megabytes; retain only a small preview while computing
// the response hash incrementally. Large bodies are omitted from the log.
const maxLoggedBodyBytes = 4 << 10

// Keep one extra byte to distinguish an oversized body from an exact fit.
// Always report the full write so the logger never truncates the transfer.
type boundedLogWriter struct{ buf *bytes.Buffer }

func (w boundedLogWriter) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := maxLoggedBodyBytes + 1 - w.buf.Len(); remaining > 0 {
		_, _ = w.buf.Write(p[:min(remaining, len(p))])
	}
	return n, nil
}

type loggedReadCloser struct {
	io.Reader
	io.Closer
}

func NewLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		var requestBody []byte
		var streamed bytes.Buffer
		if raisedBodySizePath(r.URL.Path) {
			// The upload handler must run before the body is read so it can
			// extend the connection deadline and enforce its streaming limit.
			r.Body = loggedReadCloser{io.TeeReader(r.Body, boundedLogWriter{&streamed}), r.Body}
		} else {
			requestBody, _ = io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		wWrapper := NewResponseWriterWrapper(w)
		next.ServeHTTP(wWrapper, r)
		if raisedBodySizePath(r.URL.Path) {
			requestBody = streamed.Bytes()
		}
		requestLatency := time.Since(requestStart)
		responseBody := wWrapper.Body.Bytes()
		var hash [64]byte
		if r.Method != http.MethodHead && len(responseBody) > 0 {
			copy(hash[:], wWrapper.bodyHash.Sum(nil))
		}
		if len(requestBody) > maxLoggedBodyBytes {
			requestBody = nil
		}
		if len(responseBody) > maxLoggedBodyBytes {
			responseBody = []byte{}
		}
		url := r.URL
		if r.URL.Scheme == "" {
			r.URL.Scheme = "https"
		}
		if r.URL.Host == "" {
			r.URL.Host = r.Host
		}
		req := request.NewWrite(request.Read{
			In: request.In{
				Base: request.Base{
					Body:    serverCommunication.Body{Body: requestBody},
					Headers: r.Header,
				},
				Uptime: serverCommunication.Uptime{Uptime: logger.Uptime(&requestStart)},
				Sender: serverCommunication.Sender{Sender: r.RemoteAddr},
				Url:    url,
				Method: r.Method,
			},
			Out: request.Out{
				Base: request.Base{
					Body:    serverCommunication.Body{Body: responseBody},
					Headers: wWrapper.Header(),
				},
				BodyHash:   serverCommunication.BodyHash{BodyHash: hash},
				StatusCode: wWrapper.StatusCode,
				Latency:    requestLatency,
			},
		})
		logger.CommBuffer.Log(req)
	})
}
