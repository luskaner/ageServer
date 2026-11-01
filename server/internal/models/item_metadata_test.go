package models

import (
	"encoding/json"
	"testing"
)

func TestItemMetadataAttributeRoundTrip(t *testing.T) {
	raw := []byte(`"{\"att\":{\"foo\":{\"val\":\"bar\",\"source\":2,\"mtype\":3}},\"icon\":\"x\"}"`)
	md := NewItemMetadataRaw(raw)

	if got, ok := md.GetAttribute("foo"); !ok || got != "bar" {
		t.Fatalf("GetAttribute = %q,%v; want bar,true", got, ok)
	}

	if !md.UpdateAttribute("foo", "baz") {
		t.Fatalf("UpdateAttribute reassigned attribute")
	}
	if md.UpdateAttribute("is_new", "1") {
		t.Fatal("is_new accepted")
	}
	if got, _ := md.GetAttribute("foo"); got != "baz" {
		t.Fatalf("updated value = %q; want baz", got)
	}
	if got, ok := md.GetAttribute("is_new"); ok || got != "" {
		t.Fatalf("is_new stored: %q,%v; want empty,false", got, ok)
	}

	b, err := json.Marshal(&md)
	if err != nil {
		t.Fatal(err)
	}

	var restored ItemMetadata
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if got, _ := restored.GetAttribute("foo"); got != "baz" {
		t.Fatalf("restored value = %q; want baz", got)
	}
	attr, ok := restored.attributes["foo"]
	if !ok || attr.Source != 2 || attr.Mtype != 3 {
		t.Fatalf("restored attr = %#v; want source=2,mtype=3", attr)
	}

	// Legacy documents persist attributes as a plain key -> value map.
	var legacy ItemMetadata
	if err := json.Unmarshal([]byte(`{"attributes":{"legacy":"value"}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if got, _ := legacy.GetAttribute("legacy"); got != "value" {
		t.Fatalf("legacy value = %q; want value", got)
	}
	if a := legacy.attributes["legacy"]; a.Source != 1 || a.Mtype != 1 {
		t.Fatalf("legacy attr = %#v; want source=1,mtype=1", a)
	}

	var wire map[string]any
	if err := json.Unmarshal([]byte(md.Encode()), &wire); err != nil {
		t.Fatal(err)
	}
	att, ok := wire["att"].(map[string]any)
	if !ok {
		t.Fatalf("encoded att = %#v; want object", wire["att"])
	}
	foo, ok := att["foo"].(map[string]any)
	if !ok || foo["val"] != "baz" {
		t.Fatalf("encoded foo = %#v; want val=baz", att["foo"])
	}
	if foo["source"] != float64(2) || foo["mtype"] != float64(3) {
		t.Fatalf("encoded source/mtype = %#v; want 2/3", foo)
	}
	if _, exists := att["is_new"]; exists {
		t.Fatalf("encoded is_new present: %#v", att)
	}

	md.UpdateOther("eslot", "7")
	if md.other["eslot"] != "7" {
		t.Fatalf("eslot = %#v; want 7", md.other["eslot"])
	}
	md.RemoveOther("missing")
	if _, ok := md.other["eslot"]; !ok {
		t.Fatal("eslot removed unexpectedly")
	}
}
