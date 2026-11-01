package models

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testDataV1 struct {
	Name string `json:"name"`
}

type testData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type testUpgrader struct {
	defaultData testData
	version     uint32
}

func (u testUpgrader) Default() *testData {
	defaultData := u.defaultData
	return &defaultData
}

func (u testUpgrader) CurrentVersion() uint32 {
	if u.version == 0 {
		return 1
	}
	return u.version
}

func (u testUpgrader) ObjectOfVersion(version uint32) any {
	if version == 0 {
		return &testDataV1{}
	}
	return &testData{}
}

func (u testUpgrader) UpgradeToNextVersion(_ uint32, oldObject any) *testData {
	return &testData{Name: oldObject.(*testDataV1).Name, Count: 42}
}

// stepDataV0 and stepDataV1 are the intermediate shapes of multiStepUpgrader.
type stepDataV0 struct {
	Name string `json:"name"`
}

type stepDataV1 struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// multiStepUpgrader takes a version 0 document through two migrations.
//
// The contract is worth spelling out because it is easy to get wrong: the first
// call to UpgradeToNextVersion receives the object ObjectOfVersion produced for
// the stored version, and every call after that receives what the previous call
// returned.
type multiStepUpgrader struct{}

func (multiStepUpgrader) Default() *testData {
	return &testData{Name: "default"}
}

func (multiStepUpgrader) CurrentVersion() uint32 {
	return 2
}

func (multiStepUpgrader) ObjectOfVersion(version uint32) any {
	if version == 0 {
		return &stepDataV0{}
	}
	return &stepDataV1{}
}

func (multiStepUpgrader) UpgradeToNextVersion(oldVersion uint32, oldObject any) *testData {
	if oldVersion == 0 {
		return &testData{Name: oldObject.(*stepDataV0).Name, Count: 1}
	}
	previous := oldObject.(*testData)
	return &testData{Name: previous.Name, Count: previous.Count + 10}
}

func TestJSONUserStoreRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_76561198000000000")
	store := NewJSONUserStore(dir)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load on a missing store: %s", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("a user with no data must load as empty, got %d keys", len(loaded))
	}

	want := map[string]StoredValue{
		"auth":           {Version: 0, Data: json.RawMessage(`"2026-01-02T03:04:05Z"`)},
		"avatarMetadata": {Version: 0, Data: json.RawMessage(`{"v":1,"twr":0}`)},
	}
	for key, value := range want {
		if err = store.Put(key, value); err != nil {
			t.Fatalf("Put %q: %s", key, err)
		}
	}

	reopened := NewJSONUserStore(dir)
	loaded, err = reopened.Load()
	if err != nil {
		t.Fatalf("Load after Put: %s", err)
	}
	if len(loaded) != len(want) {
		t.Fatalf("loaded %d keys, want %d", len(loaded), len(want))
	}
	for key, value := range want {
		if loaded[key].Version != value.Version {
			t.Errorf("key %q version is %d, want %d", key, loaded[key].Version, value.Version)
		}
		if string(loaded[key].Data) != string(value.Data) {
			t.Errorf("key %q is %s, want %s", key, loaded[key].Data, value.Data)
		}
	}
}

// TestJSONUserStoreOnlyRewritesTheWrittenKey is the point of storing one file
// per key: saving an avatar stat must leave the inventory file byte for byte
// identical instead of re-encoding the whole save.
func TestJSONUserStoreOnlyRewritesTheWrittenKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_76561198000000000")
	store := NewJSONUserStore(dir)

	big := `[` + strings.TrimSuffix(strings.Repeat(`{"Id":1,"DefinitionId":2},`, 5000), ",") + `]`
	if err := store.Put("items", StoredValue{Data: json.RawMessage(big)}); err != nil {
		t.Fatalf("Put items: %s", err)
	}
	if err := store.Put("avatarStats", StoredValue{Data: json.RawMessage(`{"10000":3}`)}); err != nil {
		t.Fatalf("Put avatarStats: %s", err)
	}
	itemsBefore, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("reading items.json: %s", err)
	}

	if err = store.Put("avatarStats", StoredValue{Data: json.RawMessage(`{"10000":4}`)}); err != nil {
		t.Fatalf("Put avatarStats again: %s", err)
	}

	itemsAfter, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("re-reading items.json: %s", err)
	}
	if string(itemsBefore) != string(itemsAfter) {
		t.Error("writing avatarStats modified items.json")
	}
	if len(itemsBefore) < 100000 {
		t.Errorf("expected a large items document for this test to be meaningful, got %d bytes", len(itemsBefore))
	}
}

// TestJSONUserStoreIgnoresTempFiles makes sure a temporary file left behind by
// an interrupted write is never mistaken for a saved key.
func TestJSONUserStoreIgnoresTempFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	store := NewJSONUserStore(dir)
	if err := store.Put("auth", StoredValue{Data: json.RawMessage(`"x"`)}); err != nil {
		t.Fatalf("Put: %s", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items.json.123.tmp"), []byte(`{"metadata":{"version":0},"data":`), 0644); err != nil {
		t.Fatalf("writing a stray temp file: %s", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %s", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d keys, want only the real one: %v", len(loaded), loaded)
	}
}

// TestJSONUserStoreQuarantinesCorruptKey covers the failure that used to lock a
// player out of their account for good: one damaged key must not take the rest
// of the save with it.
func TestJSONUserStoreQuarantinesCorruptKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	store := NewJSONUserStore(dir)
	if err := store.Put("auth", StoredValue{Data: json.RawMessage(`"2026-01-02T03:04:05Z"`)}); err != nil {
		t.Fatalf("Put auth: %s", err)
	}
	if err := store.Put("items", StoredValue{Data: json.RawMessage(`{"1":{"Id":1}}`)}); err != nil {
		t.Fatalf("Put items: %s", err)
	}
	corruptPath := filepath.Join(dir, "items.json")
	if err := os.WriteFile(corruptPath, []byte(`{"metadata":{"version":0},"data":{"1":{"Id"`), 0644); err != nil {
		t.Fatalf("corrupting items.json: %s", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("a damaged key must not fail the whole Load: %s", err)
	}
	if _, ok := loaded["auth"]; !ok {
		t.Error("the healthy key was dropped along with the damaged one")
	}
	if _, ok := loaded["items"]; ok {
		t.Error("the damaged key should have been reset, not returned")
	}
	if _, err = os.Stat(corruptPath + ".corrupt"); err != nil {
		t.Errorf("the damaged key was not kept aside for inspection: %s", err)
	}
	if _, err = os.Stat(corruptPath); !os.IsNotExist(err) {
		t.Error("the damaged key should no longer be in the way of the default value")
	}
}

func TestJSONUserStoreMigratesLegacyFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "STEAM_76561198000000000")
	legacyPath := dir + ".json"

	legacy := `{"metadata":{"version":0},"data":{` +
		`"auth":{"metadata":{"version":0},"data":"2026-01-02T03:04:05Z"},` +
		`"items":{"metadata":{"version":0},"data":{"1":{"Id":1}}},` +
		`"avatarStats":{"metadata":{"version":0},"data":{"10000":7}}}}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("writing the legacy save: %s", err)
	}

	store := NewJSONUserStore(dir)
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after migrating: %s", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("migrated %d keys, want 3: %v", len(loaded), loaded)
	}
	if string(loaded["items"].Data) != `{"1":{"Id":1}}` {
		t.Errorf("items is %s", loaded["items"].Data)
	}
	if string(loaded["avatarStats"].Data) != `{"10000":7}` {
		t.Errorf("avatarStats is %s", loaded["avatarStats"].Data)
	}
	if _, err = os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("the legacy file should have been retired once every key was written")
	}
	if _, err = os.Stat(legacyPath + ".migrated"); err != nil {
		t.Errorf("the legacy file should have been kept as .migrated: %s", err)
	}

	// Opening again must not resurrect or duplicate anything.
	loaded, err = NewJSONUserStore(dir).Load()
	if err != nil {
		t.Fatalf("second Load: %s", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("second Load returned %d keys, want 3", len(loaded))
	}
}

// memoryUserStore proves the seam is self-sufficient: a store that is nothing
// but a map drives PersistentStringJsonMap exactly like the files do.
type memoryUserStore struct {
	values map[string]StoredValue
	puts   int
	loads  int
}

func (m *memoryUserStore) Load() (map[string]StoredValue, error) {
	m.loads++
	loaded := make(map[string]StoredValue, len(m.values))
	for key, value := range m.values {
		loaded[key] = value
	}
	return loaded, nil
}

func (m *memoryUserStore) Put(key string, value StoredValue) error {
	m.puts++
	m.values[key] = value
	return nil
}

func TestPersistentStringJsonMapStoresDefaultsOnFirstUse(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	upgrader := testUpgrader{defaultData: testData{Name: "fresh", Count: 1}}

	if _, err = NewPersistentJsonData[*testData](persistentMap, "profile", upgrader); err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if store.puts != 1 {
		t.Errorf("expected the default value to be written once, got %d writes", store.puts)
	}
	if string(store.values["profile"].Data) != `{"name":"fresh","count":1}` {
		t.Errorf("stored %s", store.values["profile"].Data)
	}
}

func TestPersistentStringJsonMapReopensStoredValue(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		// Already at the current version, so opening it must not rewrite it.
		"profile": {Version: 1, Data: json.RawMessage(`{"name":"returned","count":9}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadOnly(func(d *testData) error {
		if d.Name != "returned" || d.Count != 9 {
			t.Errorf("read back %+v", *d)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
	if store.puts != 0 {
		t.Errorf("reopening an unchanged key must not write, got %d writes", store.puts)
	}
}

func TestPersistentStringJsonMapUpgradesStoredValue(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"profile": {Version: 0, Data: json.RawMessage(`{"name":"old"}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadOnly(func(d *testData) error {
		if d.Name != "old" || d.Count != 42 {
			t.Errorf("upgrade produced %+v, want the name kept and the field filled", *d)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
	if string(store.values["profile"].Data) != `{"name":"old","count":42}` {
		t.Errorf("the upgraded value was not persisted: %s", store.values["profile"].Data)
	}
	if store.values["profile"].Version != 1 {
		t.Errorf("persisted version is %d, want 1", store.values["profile"].Version)
	}
}

func TestPersistentStringJsonMapRejectsNewerVersion(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"profile": {Version: 99, Data: json.RawMessage(`{"name":"future"}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	if _, err = NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{}); err == nil {
		t.Error("a save written by a newer server must be rejected, not silently downgraded")
	}
}

// TestPersistentStringJsonMapWithReadWriteWritesOnlyItsKey checks the saving
// path end to end against real files.
func TestPersistentStringJsonMapWithReadWriteWritesOnlyItsKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_76561198000000000")
	persistentMap, err := NewPersistentStringMap(NewJSONUserStore(dir))
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	upgrader := testUpgrader{defaultData: testData{Name: "player", Count: 0}}

	if _, err = NewPersistentJsonData[*testData](persistentMap, "items", testUpgrader{defaultData: testData{Name: strings.Repeat("x", 200000)}}); err != nil {
		t.Fatalf("NewPersistentJsonData items: %s", err)
	}
	stats, err := NewPersistentJsonData[*testData](persistentMap, "avatarStats", upgrader)
	if err != nil {
		t.Fatalf("NewPersistentJsonData avatarStats: %s", err)
	}

	itemsBefore, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("reading items.json: %s", err)
	}
	if err = stats.WithReadWrite(func(d *testData) error {
		d.Count = 1234
		return nil
	}); err != nil {
		t.Fatalf("WithReadWrite: %s", err)
	}
	itemsAfter, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("re-reading items.json: %s", err)
	}
	if string(itemsBefore) != string(itemsAfter) {
		t.Error("saving avatarStats rewrote the untouched items document")
	}

	// And the value has to survive a restart.
	reopened, err := NewPersistentStringMap(NewJSONUserStore(dir))
	if err != nil {
		t.Fatalf("reopening: %s", err)
	}
	reloadedStats, err := NewPersistentJsonData[*testData](reopened, "avatarStats", upgrader)
	if err != nil {
		t.Fatalf("reloading avatarStats: %s", err)
	}
	if err = reloadedStats.WithReadOnly(func(d *testData) error {
		if d.Count != 1234 {
			t.Errorf("after a restart Count is %d, want 1234", d.Count)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly after restart: %s", err)
	}
}

func TestPersistentStringJsonMapRejectsDuplicateKey(t *testing.T) {
	persistentMap, err := NewPersistentStringMap(&memoryUserStore{values: map[string]StoredValue{}})
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	if _, err = NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{}); err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if _, err = NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{}); err == nil {
		t.Error("registering the same key twice must be reported")
	}
}

func TestWriteFileAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	if err := writeFileAtomic(path, []byte(`{"first":true}`)); err != nil {
		t.Fatalf("first write: %s", err)
	}
	if err := writeFileAtomic(path, []byte(`{"second":true}`)); err != nil {
		t.Fatalf("second write: %s", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %s", err)
	}
	if string(contents) != `{"second":true}` {
		t.Errorf("file holds %s", contents)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing the directory: %s", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only the target file to remain, found %d entries", len(entries))
	}
}

// --- Upgrades across versions ------------------------------------------------

// TestUpgradeAcrossMultipleVersions pins that every step between the stored and
// the current version runs, in order. A single-step upgrader would leave the
// document at Count 1.
func TestUpgradeAcrossMultipleVersions(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"profile": {Version: 0, Data: json.RawMessage(`{"name":"old"}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "profile", multiStepUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadOnly(func(d *testData) error {
		if d.Name != "old" {
			t.Errorf("name is %q, want the original kept through both steps", d.Name)
		}
		if d.Count != 11 {
			t.Errorf("Count is %d, want 11: both migration steps must have run", d.Count)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
	stored := store.values["profile"]
	if stored.Version != 2 {
		t.Errorf("persisted version is %d, want 2", stored.Version)
	}
	if string(stored.Data) != `{"name":"old","count":11}` {
		t.Errorf("persisted %s", stored.Data)
	}
}

// TestUpgradedValueStaysUpgradedOnWrite checks the version survives a later
// edit: writing after an upgrade must stamp the new version, not the old one.
func TestUpgradedValueStaysUpgradedOnWrite(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"profile": {Version: 0, Data: json.RawMessage(`{"name":"old"}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "profile", multiStepUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadWrite(func(d *testData) error {
		d.Count = 99
		return nil
	}); err != nil {
		t.Fatalf("WithReadWrite: %s", err)
	}
	if store.values["profile"].Version != 2 {
		t.Errorf("version after the edit is %d, want 2", store.values["profile"].Version)
	}
	if string(store.values["profile"].Data) != `{"name":"old","count":99}` {
		t.Errorf("persisted %s", store.values["profile"].Data)
	}

	// And reopening must not run the migrations a second time.
	reopened, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("reopening: %s", err)
	}
	putCount := store.puts
	if _, err = NewPersistentJsonData[*testData](reopened, "profile", multiStepUpgrader{}); err != nil {
		t.Fatalf("reloading: %s", err)
	}
	if store.puts != putCount {
		t.Errorf("an already current document was rewritten on open (%d extra writes)", store.puts-putCount)
	}
}

// TestUnupgradableValueIsReportedAndLeftAlone covers a document that parses as
// JSON but does not fit the shape it claims to be: the upgrade fails, the caller
// is told, and nothing is overwritten.
func TestUnupgradableValueIsReportedAndLeftAlone(t *testing.T) {
	original := StoredValue{Version: 0, Data: json.RawMessage(`"a bare string"`)}
	store := &memoryUserStore{values: map[string]StoredValue{"profile": original}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "profile", testUpgrader{})
	if err == nil {
		t.Fatal("an unupgradable document must be reported, not silently defaulted")
	}
	if data != nil {
		t.Error("no data handle should be returned when the upgrade fails")
	}
	if string(store.values["profile"].Data) != string(original.Data) || store.values["profile"].Version != 0 {
		t.Errorf("the unreadable document was modified: %+v", store.values["profile"])
	}
}

// TestNewerVersionIsRejectedPerKey checks the guard is enforced per document,
// so one document from a future version does not block the rest of the account.
func TestNewerVersionIsRejectedPerKey(t *testing.T) {
	store := &memoryUserStore{values: map[string]StoredValue{
		"future": {Version: 99, Data: json.RawMessage(`{"name":"tomorrow"}`)},
		"auth":   {Version: 1, Data: json.RawMessage(`{"name":"today"}`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	if _, err = NewPersistentJsonData[*testData](persistentMap, "future", testUpgrader{}); err == nil {
		t.Error("a document from a newer server must be rejected")
	}
	usable, err := NewPersistentJsonData[*testData](persistentMap, "auth", testUpgrader{})
	if err != nil {
		t.Fatalf("the other key must still be usable: %s", err)
	}
	if err = usable.WithReadOnly(func(d *testData) error {
		if d.Name != "today" {
			t.Errorf("read %q, want today", d.Name)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
}

// TestNewerVersionIsNotQuarantined guards a decision: a document the server
// understands but is too new for is kept on disk, so downgrading and upgrading
// again does not destroy the player's data.
func TestNewerVersionIsNotQuarantined(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	if err := NewJSONUserStore(dir).Put("avatarStats", StoredValue{Version: 99, Data: json.RawMessage(`{"10000":1}`)}); err != nil {
		t.Fatalf("Put: %s", err)
	}
	loaded, err := NewJSONUserStore(dir).Load()
	if err != nil {
		t.Fatalf("Load: %s", err)
	}
	if loaded["avatarStats"].Version != 99 {
		t.Errorf("the future document was altered: %+v", loaded["avatarStats"])
	}
	if _, err = os.Stat(filepath.Join(dir, "avatarStats.json.corrupt")); err == nil {
		t.Error("a merely newer document must not be treated as damaged")
	}
}

// --- Interrupted writes ------------------------------------------------------

// TestWriteFileAtomicNeverExposesAPartialDocument is the reason for writing to
// a temporary file: with the old truncate-then-stream approach a reader could
// catch the file mid-write.
//
// A write that is refused outright also counts as a pass, and does happen: on
// Windows a reader holding the file open makes the rename fail, because Go
// opens files without FILE_SHARE_DELETE. Refusing the change is the safe
// outcome; leaving half a document behind is the one that must never happen.
func TestWriteFileAtomicNeverExposesAPartialDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	document := func(fill string, size int) []byte {
		return []byte(`{"payload":"` + strings.Repeat(fill, size) + `"}`)
	}
	if err := writeFileAtomic(path, document("a", 300000)); err != nil {
		t.Fatalf("first write: %s", err)
	}

	type reading struct {
		partial   error
		succeeded int
	}
	stop := make(chan struct{})
	done := make(chan reading, 1)
	go func() {
		var got reading
		for {
			select {
			case <-stop:
				done <- got
				return
			default:
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var decoded struct {
				Payload string `json:"payload"`
			}
			if err = json.Unmarshal(contents, &decoded); err != nil {
				if got.partial == nil {
					got.partial = err
				}
				continue
			}
			got.succeeded++
		}
	}()

	var refused int
	for i := range 40 {
		if err := writeFileAtomic(path, document("b", 300000+i)); err != nil {
			refused++
		}
	}
	close(stop)
	got := <-done
	if got.partial != nil {
		t.Errorf("a reader observed a partially written document: %s", got.partial)
	}
	if got.succeeded == 0 {
		t.Error("the reader never managed to read the file, so nothing was verified")
	}
	t.Logf("%d complete reads, %d writes applied, %d refused", got.succeeded, 40-refused, refused)
}

// TestWriteFileAtomicFailedWriteLeavesNoDebris checks a write that cannot
// complete leaves the previous document alone and removes its temporary file.
func TestWriteFileAtomicFailedWriteLeavesNoDebris(t *testing.T) {
	dir := t.TempDir()
	// A directory where the document belongs makes the rename fail on every
	// platform.
	path := filepath.Join(dir, "doc.json")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Fatalf("MkdirAll: %s", err)
	}
	if err := writeFileAtomic(path, []byte(`{"payload":"x"}`)); err == nil {
		t.Skip("renaming onto a directory unexpectedly succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing: %s", err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		t.Errorf("expected only the target to remain, found %v", names)
	}
}

// TestJSONUserStorePutLeavesPreviousDocumentOnFailure checks a failed write
// never damages what was already stored.
func TestJSONUserStorePutLeavesPreviousDocumentOnFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	store := NewJSONUserStore(dir)
	if err := store.Put("items", StoredValue{Data: json.RawMessage(`{"1":{"Id":1}}`)}); err != nil {
		t.Fatalf("Put: %s", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("reading: %s", err)
	}

	// A value that cannot be marshalled must fail before anything is written.
	if err = store.Put("items", StoredValue{Data: json.RawMessage(`{`)}); err == nil {
		t.Log("malformed raw JSON was accepted; relying on the rename-only guarantee")
	}
	after, err := os.ReadFile(filepath.Join(dir, "items.json"))
	if err != nil {
		t.Fatalf("re-reading: %s", err)
	}
	if string(before) != string(after) {
		t.Error("a rejected write modified the stored document")
	}
}

// --- Damaged saves -----------------------------------------------------------

// TestJSONUserStoreQuarantinesDocumentlessFile covers valid JSON that carries no
// document, e.g. a write that was interrupted and happened to end on a brace.
func TestJSONUserStoreQuarantinesDocumentlessFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("MkdirAll: %s", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("writing: %s", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"metadata":{"version":0},"data":"2026-01-02T03:04:05Z"}`), 0644); err != nil {
		t.Fatalf("writing auth: %s", err)
	}
	loaded, err := NewJSONUserStore(dir).Load()
	if err != nil {
		t.Fatalf("Load: %s", err)
	}
	if _, ok := loaded["items"]; ok {
		t.Error("a file with no document must not be returned as a value")
	}
	if _, ok := loaded["auth"]; !ok {
		t.Error("the healthy key was dropped")
	}
	if _, err = os.Stat(filepath.Join(dir, "items.json.corrupt")); err != nil {
		t.Errorf("the documentless file was not set aside: %s", err)
	}
}

// TestJSONUserStoreQuarantinesCorruptLegacyFile is the counterpart of the case
// above: the legacy single file holds the whole account, so failing to read it
// must not lock the player out.
func TestJSONUserStoreQuarantinesCorruptLegacyFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "STEAM_76561198000000000")
	legacyPath := dir + ".json"
	if err := os.WriteFile(legacyPath, []byte(`{"metadata":{"version":0},"data":{"items":`), 0644); err != nil {
		t.Fatalf("writing: %s", err)
	}
	loaded, err := NewJSONUserStore(dir).Load()
	if err != nil {
		t.Fatalf("a damaged legacy save must not fail the whole Load: %s", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected to start empty, got %v", loaded)
	}
	if _, err = os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("the damaged legacy file should have been moved out of the way")
	}
	if _, err = os.Stat(legacyPath + ".corrupt"); err != nil {
		t.Errorf("the damaged legacy file was not kept for inspection: %s", err)
	}
	// And the user has to be usable afterwards.
	if err = NewJSONUserStore(dir).Put("auth", StoredValue{Data: json.RawMessage(`"x"`)}); err != nil {
		t.Fatalf("Put after recovering: %s", err)
	}
	if loaded, err = NewJSONUserStore(dir).Load(); err != nil || len(loaded) != 1 {
		t.Errorf("the user must be usable after a damaged legacy save: %v %v", loaded, err)
	}
}

// --- Keys and shapes ---------------------------------------------------------

func TestJSONUserStoreEscapesAwkwardKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_1")
	store := NewJSONUserStore(dir)
	want := map[string]StoredValue{
		"plain":          {Data: json.RawMessage(`{"a":1}`)},
		"with/slash":     {Data: json.RawMessage(`{"b":2}`)},
		"with%percent":   {Data: json.RawMessage(`{"c":3}`)},
		"with spaces ..": {Data: json.RawMessage(`{"d":4}`)},
	}
	for key, value := range want {
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put %q: %s", key, err)
		}
	}
	loaded, err := NewJSONUserStore(dir).Load()
	if err != nil {
		t.Fatalf("Load: %s", err)
	}
	if len(loaded) != len(want) {
		t.Fatalf("loaded %d keys, want %d: %v", len(loaded), len(want), loaded)
	}
	for key, value := range want {
		if string(loaded[key].Data) != string(value.Data) {
			t.Errorf("key %q came back as %s", key, loaded[key].Data)
		}
	}
}

// TestPersistentStringJsonMapRealGameTypes runs the types the games actually
// register, rather than the test stand-ins.
func TestPersistentStringJsonMapRealGameTypes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "STEAM_76561198000000000")
	persistentMap, err := NewPersistentStringMap(NewJSONUserStore(dir))
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	auth, err := NewPersistentJsonData(persistentMap, "auth", NewAuthUpgradableDefaultData())
	if err != nil {
		t.Fatalf("NewPersistentJsonData auth: %s", err)
	}
	metadata, err := NewPersistentJsonData(persistentMap, "avatarMetadata", NewAvatarMetadataUpgradableDefaultData("age4"))
	if err != nil {
		t.Fatalf("NewPersistentJsonData avatarMetadata: %s", err)
	}

	wantAuth := time.Date(2026, 8, 3, 19, 39, 13, 0, time.UTC)
	if err = auth.WithReadWrite(func(d *time.Time) error {
		*d = wantAuth
		return nil
	}); err != nil {
		t.Fatalf("writing auth: %s", err)
	}
	wantMetadata := `{"sharedHistory":1,"hardwareType":2,"inputDeviceType":0}`
	if err = metadata.WithReadWrite(func(d *string) error {
		*d = wantMetadata
		return nil
	}); err != nil {
		t.Fatalf("writing avatarMetadata: %s", err)
	}

	reopened, err := NewPersistentStringMap(NewJSONUserStore(dir))
	if err != nil {
		t.Fatalf("reopening: %s", err)
	}
	reloadedAuth, err := NewPersistentJsonData(reopened, "auth", NewAuthUpgradableDefaultData())
	if err != nil {
		t.Fatalf("reloading auth: %s", err)
	}
	if err = reloadedAuth.WithReadOnly(func(d *time.Time) error {
		if !d.Equal(wantAuth) {
			t.Errorf("auth came back as %s, want %s", d, wantAuth)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading auth: %s", err)
	}
	reloadedMetadata, err := NewPersistentJsonData(reopened, "avatarMetadata", NewAvatarMetadataUpgradableDefaultData("age4"))
	if err != nil {
		t.Fatalf("reloading avatarMetadata: %s", err)
	}
	if err = reloadedMetadata.WithReadOnly(func(d *string) error {
		if *d != wantMetadata {
			t.Errorf("avatarMetadata came back as %s", *d)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading avatarMetadata: %s", err)
	}
}

// --- Concurrency -------------------------------------------------------------

// TestPersistentStringJsonMapConcurrentAccess exercises the two caches at once:
// different keys are mutated and read from different goroutines, and the same
// key is hit by two of them. Run it with -race.
func TestPersistentStringJsonMapConcurrentAccess(t *testing.T) {
	persistentMap, err := NewPersistentStringMap(NewJSONUserStore(filepath.Join(t.TempDir(), "STEAM_1")))
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	keys := []string{"avatarStats", "items", "profileProperties", "itemLoadouts"}
	data := make(map[string]*PersistentJsonData[*testData], len(keys))
	for _, key := range keys {
		if data[key], err = NewPersistentJsonData[*testData](persistentMap, key, testUpgrader{}); err != nil {
			t.Fatalf("NewPersistentJsonData %q: %s", key, err)
		}
	}
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			key := keys[worker%len(keys)]
			// Every second worker shares a key with the previous one.
			if worker%2 == 1 {
				key = keys[(worker-1)%len(keys)]
			}
			for range 25 {
				if err := data[key].WithReadWrite(func(d *testData) error {
					d.Count++
					return nil
				}); err != nil {
					t.Errorf("WithReadWrite on %q: %s", key, err)
					return
				}
				if err := data[key].WithReadOnly(func(d *testData) error {
					if d.Count < 0 {
						t.Errorf("torn read on %q: %d", key, d.Count)
					}
					return nil
				}); err != nil {
					t.Errorf("WithReadOnly on %q: %s", key, err)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

// --- A document that could not be opened -------------------------------------

// TestUnavailableDocumentDoesNotPanic is the reason PersistentJsonData tolerates
// a nil receiver. Every route discards the error it gets back, so a nil handle
// used to reach a nil dereference inside a request goroutine, which no recover
// in the server catches, taking the whole process down.
func TestUnavailableDocumentDoesNotPanic(t *testing.T) {
	var unavailable *PersistentJsonData[*testData]
	if err := unavailable.WithReadOnly(func(*testData) error { return nil }); !errors.Is(err, ErrDataUnavailable) {
		t.Errorf("WithReadOnly returned %v, want ErrDataUnavailable", err)
	}
	if err := unavailable.WithReadWrite(func(*testData) error { return nil }); !errors.Is(err, ErrDataUnavailable) {
		t.Errorf("WithReadWrite returned %v, want ErrDataUnavailable", err)
	}
	if _, err := unavailable.MarshalJSON(); !errors.Is(err, ErrDataUnavailable) {
		t.Errorf("MarshalJSON returned %v, want ErrDataUnavailable", err)
	}
	// The callback must not run: there is nothing to hand it.
	called := false
	_ = unavailable.WithReadWrite(func(*testData) error { called = true; return nil })
	if called {
		t.Error("the callback ran on an unavailable document")
	}
	// A handle whose map is missing is treated the same way.
	orphan := &PersistentJsonData[*testData]{key: "items"}
	if err := orphan.WithReadOnly(func(*testData) error { return nil }); !errors.Is(err, ErrDataUnavailable) {
		t.Errorf("orphan handle returned %v, want ErrDataUnavailable", err)
	}
	if err := orphan.available(); err == nil || !strings.Contains(err.Error(), "items") {
		t.Errorf("the error should name the document, got %v", err)
	}
}

// TestUnavailableDocumentEncodesAsNull covers the wire format: a user carrying
// an unavailable avatarMetadata is still serialised for the game client.
func TestUnavailableDocumentEncodesAsNull(t *testing.T) {
	var unavailable *PersistentJsonData[*testData]
	encoded, err := json.Marshal(struct {
		Data *PersistentJsonData[*testData] `json:"data"`
	}{Data: unavailable})
	if err != nil {
		t.Fatalf("encoding an unavailable document: %s", err)
	}
	if string(encoded) != `{"data":null}` {
		t.Errorf("encoded as %s, want {\"data\":null}", encoded)
	}
}

// TestGenerateKeepsTheUserWhenOneDocumentCannotBeOpened is the case that used to
// be invisible: a document from a newer server leaves its field nil, the rest of
// the account still works, and using the broken one reports an error instead of
// crashing.
func TestGenerateKeepsTheUserWhenOneDocumentCannotBeOpened(t *testing.T) {
	isolateUserData(t)
	store := &memoryUserStore{values: map[string]StoredValue{
		"avatarMetadata": {Version: 99, Data: json.RawMessage(`{"v":1}`)},
		"auth":           {Data: json.RawMessage(`"2026-01-02T03:04:05Z"`)},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	users := &MainUsers{}
	users.Initialize()

	user := users.Generate("age2", persistentMap, nil, nil, "/steam/76561198000000000", false, 76561198000000000, "player")
	if user == nil {
		t.Fatal("the user must still be created when one document cannot be opened")
	}
	main, ok := user.(*MainUser)
	if !ok {
		t.Fatalf("expected a *MainUser, got %T", user)
	}
	if main.GetAvatarMetadata() != nil {
		t.Error("the unavailable document should have been left nil")
	}
	if err = main.GetAvatarMetadata().WithReadWrite(func(*string) error { return nil }); !errors.Is(err, ErrDataUnavailable) {
		t.Errorf("using the unavailable document returned %v, want ErrDataUnavailable", err)
	}
	// Everything else has to keep working.
	if main.GetAuth() == nil {
		t.Fatal("auth should still be available")
	}
	if err = main.GetAuth().WithReadOnly(func(deadline *time.Time) error {
		if deadline.IsZero() {
			t.Error("auth came back empty")
		}
		return nil
	}); err != nil {
		t.Errorf("the healthy document stopped working: %s", err)
	}
	if main.GetItemLoadouts() == nil {
		t.Error("itemLoadouts should still be available")
	}
	if main.GetAlias() != "player" {
		t.Errorf("alias is %q, want player", main.GetAlias())
	}
}

// TestGenerateOverUnavailableDocumentDoesNotOverwriteIt makes sure a document
// this server cannot understand is never replaced by defaults.
func TestGenerateOverUnavailableDocumentDoesNotOverwriteIt(t *testing.T) {
	isolateUserData(t)
	original := StoredValue{Version: 99, Data: json.RawMessage(`{"v":1,"twr":0}`)}
	store := &memoryUserStore{values: map[string]StoredValue{"avatarMetadata": original}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	users := &MainUsers{}
	users.Initialize()
	users.Generate("age4", persistentMap, nil, nil, "/steam/1", false, 1, "player")

	if string(store.values["avatarMetadata"].Data) != string(original.Data) || store.values["avatarMetadata"].Version != 99 {
		t.Errorf("the future document was rewritten: %+v", store.values["avatarMetadata"])
	}
}

// --- Write failure & flush ---------------------------------------------------

// TestPersistFailureMarksDirtyAndKeepsMemory verifies that a failed write
// leaves the updated value in memory, marks the key dirty, and returns an
// error.
func TestPersistFailureMarksDirtyAndKeepsMemory(t *testing.T) {
	store := &failingUserStore{
		values: map[string]StoredValue{
			"auth": {Data: json.RawMessage(`{"name":"old"}`), Version: 0},
		},
		failAfter: 1,
	}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "auth", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadWrite(func(d *testData) error {
		d.Count = 123
		return nil
	}); err == nil {
		t.Fatal("expected write to fail")
	}
	// The in-memory value must be updated even though the write failed.
	cached, ok := persistentMap.cachedRawData.Load("auth")
	if !ok || string(cached.Data) != `{"name":"old","count":123}` {
		t.Errorf("in-memory value not updated: %v", cached)
	}
	if !persistentMap.IsDirty("auth") {
		t.Error("key should be marked dirty after a failed write")
	}
}

// TestFlushRetriesDirtyKeysAndCleansOnSuccess checks that Flush retries the
// store write and clears the dirty flag on success.
func TestFlushRetriesDirtyKeysAndCleansOnSuccess(t *testing.T) {
	store := &failingUserStore{
		values: map[string]StoredValue{
			"auth": {Data: json.RawMessage(`{"name":"old"}`), Version: 0},
		},
		failAfter: 1,
	}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "auth", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadWrite(func(d *testData) error {
		d.Count = 456
		return nil
	}); err == nil {
		t.Fatal("expected first write to fail")
	}
	if !persistentMap.IsDirty("auth") {
		t.Error("key should be dirty after failed write")
	}

	// Now allow the write to succeed.
	store.failAfter = 2
	if err = persistentMap.Flush(); err != nil {
		t.Fatalf("Flush: %s", err)
	}
	if persistentMap.IsDirty("auth") {
		t.Error("key should be clean after successful Flush")
	}
	if string(store.values["auth"].Data) != `{"name":"old","count":456}` {
		t.Errorf("store not updated after Flush: %s", store.values["auth"].Data)
	}
}

// TestFlushReturnsErrorWhenSomeKeysStillFail checks that Flush reports an
// error if at least one key still cannot be written.
func TestFlushReturnsErrorWhenSomeKeysStillFail(t *testing.T) {
	store := &failingUserStore{
		values: map[string]StoredValue{
			"auth":  {Data: json.RawMessage(`{"name":"old"}`), Version: 0},
			"items": {Data: json.RawMessage(`[]`), Version: 0},
		},
		failAfter: 1,
	}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "auth", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if err = data.WithReadWrite(func(d *testData) error {
		d.Count = 1
		return nil
	}); err == nil {
		t.Fatal("expected write to fail")
	}
	// Flush should fail because the store still fails.
	if err = persistentMap.Flush(); err == nil {
		t.Fatal("Flush should return an error when the store still fails")
	}
	if !persistentMap.IsDirty("auth") {
		t.Error("key should remain dirty after failed Flush")
	}
}

// TestIsDirtyAndDirtyKeys reports the dirty state correctly.
func TestIsDirtyAndDirtyKeys(t *testing.T) {
	store := &failingUserStore{
		values: map[string]StoredValue{
			"auth":  {Data: json.RawMessage(`{"name":"x"}`), Version: 0},
			"items": {Data: json.RawMessage(`[]`), Version: 0},
		},
		failAfter: 1,
	}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*testData](persistentMap, "auth", testUpgrader{})
	if err != nil {
		t.Fatalf("NewPersistentJsonData: %s", err)
	}
	if persistentMap.IsDirty("auth") {
		t.Error("key should not be dirty initially")
	}
	if len(persistentMap.DirtyKeys()) != 0 {
		t.Errorf("DirtyKeys should be empty initially, got %v", persistentMap.DirtyKeys())
	}
	if err = data.WithReadWrite(func(d *testData) error { d.Count = 1; return nil }); err == nil {
		t.Fatal("expected write to fail")
	}
	if !persistentMap.IsDirty("auth") {
		t.Error("key should be dirty after failed write")
	}
	if len(persistentMap.DirtyKeys()) != 1 || persistentMap.DirtyKeys()[0] != "auth" {
		t.Errorf("DirtyKeys = %v, want [auth]", persistentMap.DirtyKeys())
	}
}

// failingUserStore is a test store that fails Put calls after the first N calls.
type failingUserStore struct {
	values    map[string]StoredValue
	putCount  int
	failAfter int // fail after this many successful puts
}

func (m *failingUserStore) Load() (map[string]StoredValue, error) {
	loaded := make(map[string]StoredValue, len(m.values))
	for key, value := range m.values {
		loaded[key] = value
	}
	return loaded, nil
}

func (m *failingUserStore) Put(key string, value StoredValue) error {
	if m.putCount >= m.failAfter {
		return errors.New("simulated disk failure")
	}
	m.putCount++
	m.values[key] = value
	return nil
}
