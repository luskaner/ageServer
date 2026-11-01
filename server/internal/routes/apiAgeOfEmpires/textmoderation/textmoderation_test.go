package textmoderation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextModerationAllow(t *testing.T) {
	body := `{"textType":"SanitisationUsername","textContent":"hello","language":"en"}`
	req := httptest.NewRequest("POST", "/textmoderation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	TextModeration(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not json: %v body=%q", err, rr.Body.String())
	}
	if resp["filterResult"] != "Allow" {
		t.Fatalf("filterResult = %v", resp["filterResult"])
	}
}

func TestTextModerationBadRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/textmoderation", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	TextModeration(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

func TestTextModerationNonUsernameNoOutput(t *testing.T) {
	body := `{"textType":"Other","textContent":"hello"}`
	req := httptest.NewRequest("POST", "/textmoderation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	TextModeration(rr, req)
	// Con textType distinto no escribe respuesta; no debe panicar.
	if rr.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rr.Body.String())
	}
}
