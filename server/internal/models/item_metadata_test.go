package models

import (
	"encoding/json"
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
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
	// is_new is accepted: the client reports the item as seen through it.
	if !md.UpdateAttribute(isNewAttributeKey, isNewAttributeSeenValue) {
		t.Fatal("is_new rejected")
	}
	if got, _ := md.GetAttribute("foo"); got != "baz" {
		t.Fatalf("updated value = %q; want baz", got)
	}
	if got, ok := md.GetAttribute(isNewAttributeKey); !ok || got != isNewAttributeSeenValue {
		t.Fatalf("is_new stored: %q,%v; want %q,true", got, ok, isNewAttributeSeenValue)
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
	// The seen flag survives a save and load: otherwise the client would show
	// the item as new again after a restart.
	if got, ok := restored.GetAttribute(isNewAttributeKey); !ok || got != isNewAttributeSeenValue {
		t.Fatalf("restored is_new = %q,%v; want %q,true", got, ok, isNewAttributeSeenValue)
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
	// is_new is sent as the official service does: the value alone, without
	// the source/mtype the definitions declare.
	isNew, ok := att[isNewAttributeKey].(map[string]any)
	if !ok || isNew["val"] != isNewAttributeSeenValue {
		t.Fatalf("encoded is_new = %#v; want val=%q", att[isNewAttributeKey], isNewAttributeSeenValue)
	}
	if _, exists := isNew["source"]; exists {
		t.Fatalf("encoded is_new carries source: %#v", isNew)
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

// testRewardItems builds an Items with two definitions: one that declares
// is_new (a reward) and one that does not.
func testRewardItems(t *testing.T) *MainItems {
	t.Helper()
	definitions := []byte(`{
		"itemCategories": [{"categoryID": 1, "name": "Portrait", "categoryGroup": "Portrait"}],
		"itemDefinitions": [
			{
				"id": 1001,
				"name": "reward",
				"categoryIDs": [1],
				"metaData": "{\"att\":{\"is_new\":{\"val\":\"1\",\"source\":1,\"mtype\":1}},\"icon\":\"x\"}"
			},
			{
				"id": 1002,
				"name": "plain",
				"categoryIDs": [1],
				"metaData": "{\"icon\":\"y\"}"
			}
		]
	}`)
	items := &MainItems{}
	items.Initialize(definitions, i.A{})
	if _, ok := items.GetDefinition(1001); !ok {
		t.Fatal("reward definition was not loaded")
	}
	return items
}

// Items handed out by default must not show the new badge: the client would
// otherwise flag every reward the profile owns.
func TestDefaultItemsAreMarkedSeen(t *testing.T) {
	i.InitializeRng(42)
	upgrader := NewItemsUpgradableDefaultData("age4", testRewardItems(t))
	defaults := upgrader.Default()
	if len(*defaults) != 2 {
		t.Fatalf("got %d items, want 2", len(*defaults))
	}
	for _, item := range *defaults {
		seen, ok := item.GetMetadata().GetAttribute(isNewAttributeKey)
		switch item.itemDefinitionID {
		case 1001:
			if !ok || seen != isNewAttributeSeenValue {
				t.Fatalf("reward item is_new = %q,%v; want %q,true", seen, ok, isNewAttributeSeenValue)
			}
		case 1002:
			if ok {
				t.Fatalf("plain item got is_new = %q", seen)
			}
		default:
			t.Fatalf("unexpected definition %d", item.itemDefinitionID)
		}
	}
}

// Profiles saved by the previous build have no is_new attribute on their
// reward items, so the upgrade must mark them as seen.
func TestItemsUpgradeMarksOwnedRewardsSeen(t *testing.T) {
	upgrader := NewItemsUpgradableDefaultData("age4", testRewardItems(t))
	if upgrader.CurrentVersion() < 1 {
		t.Fatalf("unexpected version %d", upgrader.CurrentVersion())
	}
	stored := &map[int32]*MainItem{
		1: {id: 1, itemDefinitionID: 1001},
		2: {id: 2, itemDefinitionID: 1002},
	}
	upgraded := upgrader.UpgradeToNextVersion(0, stored)
	if upgraded == nil || len(*upgraded) != 2 {
		t.Fatalf("upgrade produced %#v", upgraded)
	}
	if seen, ok := (*upgraded)[1].GetMetadata().GetAttribute(isNewAttributeKey); !ok || seen != isNewAttributeSeenValue {
		t.Fatalf("reward item is_new = %q,%v; want %q,true", seen, ok, isNewAttributeSeenValue)
	}
	if seen, ok := (*upgraded)[2].GetMetadata().GetAttribute(isNewAttributeKey); ok {
		t.Fatalf("plain item got is_new = %q", seen)
	}
}

// The upgrade must also run when the document is opened through the persistent
// store, which is the path a profile saved by the previous build takes.
func TestItemsStoredVersion0IsMigrated(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"items": {
			Version: 0,
			Data: json.RawMessage(`{
				"1": {"Id": 1, "DefinitionId": 1001, "Metadata": {}, "CreationDate": "2026-01-01T00:00:00Z"},
				"2": {"Id": 2, "DefinitionId": 1002, "Metadata": {}, "CreationDate": "2026-01-01T00:00:00Z"}
			}`),
		},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*map[int32]*MainItem](
		persistentMap,
		"items",
		NewItemsUpgradableDefaultData("age4", testRewardItems(t)),
	)
	if err != nil {
		t.Fatalf("opening stored items: %s", err)
	}
	if err = data.WithReadOnly(func(items *map[int32]*MainItem) error {
		if len(*items) != 2 {
			t.Fatalf("got %d items, want 2", len(*items))
		}
		if seen, ok := (*items)[1].GetMetadata().GetAttribute(isNewAttributeKey); !ok || seen != isNewAttributeSeenValue {
			t.Fatalf("reward item is_new = %q,%v; want %q,true", seen, ok, isNewAttributeSeenValue)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
	storedValue, exists := store.values["items"]
	if !exists {
		t.Fatal("the migrated document was not persisted")
	}
	if storedValue.Version != itemsVersion {
		t.Fatalf("stored version %d, want %d", storedValue.Version, itemsVersion)
	}
}
