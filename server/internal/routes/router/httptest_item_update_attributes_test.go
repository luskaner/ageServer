package router

import (
	"encoding/json"
	"net/http"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
)

func TestUpdateItemAttributes_NestedArrays(t *testing.T) {
	g := newTestGame(t, "age4")
	h := gameHandler("age4")
	sid := createSession(t, g)

	body := `attributeKeys=[["sigil_pos"],["sigil_pos"]]&attributeValues=[["001_centre_large_single"],["001_centre_large_single"]]&itemInstance_ids=[12345,67890]&xpGains=[0,0]`
	rr := doRequest(h, "POST", "/game/item/updateItemAttributes", g, sid, body, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var arr i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatal(err)
	}
	if arr[0] != float64(0) {
		t.Fatalf("result = %v; want 0 body=%s", arr[0], rr.Body.String())
	}
	errorCodes, ok := arr[1].([]any)
	if !ok || len(errorCodes) != 2 {
		t.Fatalf("errorCodes = %#v; want 2 items", arr[1])
	}
	for i, item := range errorCodes {
		code, _ := item.([]any)
		if len(code) != 2 || code[0] != float64(2) {
			t.Fatalf("item %d error = %#v; want [2, [2]]", i, item)
		}
	}
	itemsEncoded, _ := arr[2].([]any)
	if len(itemsEncoded) != 2 {
		t.Fatalf("itemsEncoded = %#v; want 2", arr[2])
	}

	// mismatched top-level array counts must be rejected before touching items.
	badBody := `attributeKeys=[["sigil_pos"]]&attributeValues=[["x"],["y"]]&itemInstance_ids=[12345,67890]&xpGains=[0,0]`
	rr = doRequest(h, "POST", "/game/item/updateItemAttributes", g, sid, badBody, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var badArr i.A
	if err := json.Unmarshal(rr.Body.Bytes(), &badArr); err != nil {
		t.Fatal(err)
	}
	if badArr[0] != float64(2) {
		t.Fatalf("bad result = %v; want 2", badArr[0])
	}
}
