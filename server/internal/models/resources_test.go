package models

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	i "github.com/luskaner/ageServer/server/internal"
)

func writer(w *httptest.ResponseRecorder) *http.ResponseWriter {
	var rw http.ResponseWriter = w
	return &rw
}

func signedAssetRequest(signature string) *http.Request {
	return &http.Request{URL: &url.URL{RawQuery: "signature=" + signature}}
}

// A keyed asset that never made it into keyedFiles used to be served as an
// empty 200, which the client reads as an empty catalogue rather than as a
// broken install. It has to be an internal error.
func TestReturnSignedAsset_MissingKeyedAsset(t *testing.T) {
	resources := &MainResources{
		keyedFiles:      map[string][]byte{},
		nameToSignature: map[string]string{},
		arrayFiles:      map[string]i.A{},
	}
	for _, name := range []string{"itemDefinitions.json", "never-signed.json"} {
		w := httptest.NewRecorder()
		resources.ReturnSignedAsset(name, writer(w), signedAssetRequest(""), true)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("%s: code = %d, want 500", name, w.Code)
		}
		if w.Body.Len() == 0 {
			t.Fatalf("%s: the error must explain which asset is missing", name)
		}
	}
}

// A body with no signature cannot be verified either, so it must not go out as
// if it were a valid asset.
func TestReturnSignedAsset_KeyedAssetWithoutSignature(t *testing.T) {
	resources := &MainResources{
		keyedFiles:      map[string][]byte{"itemDefinitions.json": []byte("{}")},
		nameToSignature: map[string]string{},
		arrayFiles:      map[string]i.A{},
	}
	w := httptest.NewRecorder()
	resources.ReturnSignedAsset("itemDefinitions.json", writer(w), signedAssetRequest(""), true)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", w.Code)
	}
}

// Same for an array asset, including the case where the trailing element is
// not a signature at all, which used to panic on the type assertion.
func TestReturnSignedAsset_MissingArrayAsset(t *testing.T) {
	resources := &MainResources{
		keyedFiles:      map[string][]byte{},
		nameToSignature: map[string]string{},
		arrayFiles:      map[string]i.A{"achievements.json": i.A{}},
	}
	w := httptest.NewRecorder()
	resources.ReturnSignedAsset("never-loaded.json", writer(w), signedAssetRequest(""), false)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unloaded asset: code = %d, want 500", w.Code)
	}

	w = httptest.NewRecorder()
	resources.ReturnSignedAsset("achievements.json", writer(w), signedAssetRequest(""), false)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("asset without a signature: code = %d, want 500", w.Code)
	}
}

// The happy path must be untouched: a body whose signature the client already
// holds still gets the short response.
func TestReturnSignedAsset_ServesCachedSignature(t *testing.T) {
	body := []byte(`{"result":0,"dataSignature":"c2ln"}`)
	resources := &MainResources{
		keyedFiles:      map[string][]byte{"itemDefinitions.json": body},
		nameToSignature: map[string]string{"itemDefinitions.json": "c2ln"},
		arrayFiles:      map[string]i.A{},
	}
	w := httptest.NewRecorder()
	resources.ReturnSignedAsset("itemDefinitions.json", writer(w), signedAssetRequest("c2ln"), true)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != `{"result":0,"dataSignature":"c2ln"}` {
		t.Fatalf("body = %q, want the signature-only response", got)
	}
}

// A signature the client does not hold yet still gets the full body, which is
// the flow a first launch exercises.
func TestReturnSignedAsset_ServesFullBodyForNewSignature(t *testing.T) {
	body := []byte(`{"result":0,"dataSignature":"c2ln"}`)
	resources := &MainResources{
		keyedFiles:      map[string][]byte{"itemDefinitions.json": body},
		nameToSignature: map[string]string{"itemDefinitions.json": "c2ln"},
		arrayFiles:      map[string]i.A{},
	}
	w := httptest.NewRecorder()
	resources.ReturnSignedAsset("itemDefinitions.json", writer(w), signedAssetRequest(""), true)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != string(body) {
		t.Fatalf("body = %q, want the full stamped body", got)
	}
}

const capturedBody = `{"result":0,"itemCategories":[],"itemDefinitions":[],"dataSignature":"Y2FwdHVyZWQ="}`

// responsesFor lays out a responses/<gameId> folder holding the given files and
// points the loader at it.
func responsesFor(t *testing.T, gameId string, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, gameId)
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	old := ResponsesFolder
	ResponsesFolder = dir
	t.Cleanup(func() { ResponsesFolder = old })
}

// catalogResources is MainResources in the shape Initialize would leave it in,
// minus the parts initializeResponses does not touch.
func catalogResources() *MainResources {
	return &MainResources{
		keyedFilenames:          mapset.NewSet[string]("itemDefinitions.json"),
		keyedFiles:              map[string][]byte{},
		keyedFilesCaptured:      map[string][]byte{},
		nameToSignature:         map[string]string{},
		nameToSignatureCaptured: map[string]string{},
		arrayFiles:              map[string]i.A{},
	}
}

// With signing off the captured body has to go out byte for byte: the signature
// inside it was made over exactly those bytes, so re-serialising it would
// invalidate the very thing the client checks. It also has to work with no key
// on disk at all, which is what makes it the default.
func TestInitializeResponses_ReplaysCapturedSignatureWhenNotSigning(t *testing.T) {
	responsesFor(t, "age2", map[string]string{"itemDefinitions.json": capturedBody})
	oldSign := i.SignConfigRuntime.Enabled
	i.SignConfigRuntime.Enabled = false
	t.Cleanup(func() { i.SignConfigRuntime.Enabled = oldSign })

	resources := catalogResources()
	resources.initializeResponses("age2")

	if got := string(resources.keyedFiles["itemDefinitions.json"]); got != capturedBody {
		t.Fatalf("body = %q, want the captured bytes", got)
	}
	if got := resources.nameToSignature["itemDefinitions.json"]; got != "Y2FwdHVyZWQ=" {
		t.Fatalf("signature = %q, want the captured one", got)
	}
}

// With signing on the same file must come back re-signed: a different signature,
// over a body whose dataSignature was emptied first.
func TestInitializeResponses_StampsWithOwnKeyWhenSigning(t *testing.T) {
	useTestKey(t)
	responsesFor(t, "age2", map[string]string{"itemDefinitions.json": capturedBody})
	oldSign := i.SignConfigRuntime.Enabled
	i.SignConfigRuntime.Enabled = true
	t.Cleanup(func() { i.SignConfigRuntime.Enabled = oldSign })

	resources := catalogResources()
	resources.initializeResponses("age2")

	stamped := resources.keyedFiles["itemDefinitions.json"]
	signature := resources.nameToSignature["itemDefinitions.json"]
	if signature == "" || signature == "Y2FwdHVyZWQ=" {
		t.Fatalf("signature = %q, want a freshly signed one", signature)
	}
	if got := SignatureOf(stamped); got != signature {
		t.Fatalf("body carries %q, want %q", got, signature)
	}
	if bytes.Equal(stamped, []byte(capturedBody)) {
		t.Fatal("the body was served unchanged, so nothing was re-signed")
	}
}
