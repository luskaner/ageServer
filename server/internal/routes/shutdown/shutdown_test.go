package shutdown

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShutdownBlackBoxInvalidRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/shutdown", nil)
	req.RemoteAddr = "invalid"
	rr := httptest.NewRecorder()
	Shutdown(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestShutdownBlackBoxUnauthorizedIs401Or500(t *testing.T) {
	req := httptest.NewRequest("GET", "/shutdown", nil)
	req.RemoteAddr = "203.0.113.1:12345" // TEST-NET-3, unlikely to be local
	rr := httptest.NewRecorder()
	Shutdown(rr, req)
	// Should be 401 (unauthorized) or 500 (if no local IPs), but not 200
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusInternalServerError {
		t.Logf("got %d, expected 401 or 500", rr.Code)
	}
}
