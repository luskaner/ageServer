package item

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func signItems(t *testing.T) i.A {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/game/item/signItems", strings.NewReader("itemIDs=[1,2]"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	SignItems(w, r)
	var arr i.A
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("body %q is not a JSON array: %v", w.Body.String(), err)
	}
	if len(arr) != 2 {
		t.Fatalf("body = %v, want [result, signature]", arr)
	}
	return arr
}

func wantRefusal(t *testing.T, arr i.A) {
	t.Helper()
	// Decoded JSON, so the result code arrives as a float64 like any other.
	if arr[0] != float64(2) || arr[1] != "" {
		t.Fatalf("body = %v, want [2, \"\"]", arr)
	}
}

// With signing off there is no signature to hand back, and the only honest
// answer is the one the official server gives. It used to answer with a
// hardcoded blob whatever the request said, which no client can verify.
func TestSignItems_RefusesWhenNotSigning(t *testing.T) {
	oldSign := i.SignConfigRuntime.Enabled
	i.SignConfigRuntime.Enabled = false
	t.Cleanup(func() { i.SignConfigRuntime.Enabled = oldSign })

	wantRefusal(t, signItems(t))
}

// Signing on but with no key to sign with must land on the same refusal, not
// on a well-formed signature the client cannot verify and nothing in the log
// would explain.
func TestSignItems_RefusesWhenSigningHasNoKey(t *testing.T) {
	if models.SigningKeyConfigured() {
		t.Skip("a signing key sits next to the test binary, so there is a signature to give")
	}
	oldSign := i.SignConfigRuntime.Enabled
	i.SignConfigRuntime.Enabled = true
	t.Cleanup(func() { i.SignConfigRuntime.Enabled = oldSign })

	wantRefusal(t, signItems(t))
}
