package serverStatus

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerStatusBlackBox(t *testing.T) {
	req := httptest.NewRequest("GET", "/serverStatus", nil)
	rr := httptest.NewRecorder()
	ServerStatus(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	// Test with httptest server as black box
	handler := http.HandlerFunc(ServerStatus)
	req2 := httptest.NewRequest("POST", "/serverStatus", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("POST expected 404, got %d", rr2.Code)
	}
}
