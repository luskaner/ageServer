package user

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/luskaner/ageServer/server/internal/models"
)

type mapStore struct {
	values map[string]models.StoredValue
}

func (m *mapStore) Load() (map[string]models.StoredValue, error) {
	loaded := make(map[string]models.StoredValue, len(m.values))
	for key, value := range m.values {
		loaded[key] = value
	}
	return loaded, nil
}

func (m *mapStore) Put(key string, value models.StoredValue) error {
	m.values[key] = value
	return nil
}

// TestGenerateKeepsTheUserWhenPlayfabCannotBeOpened covers a playfab document
// written by a newer server. Returning a nil user here used to lock the player
// out of AoM entirely, and made the type assertion on the base user panic.
func TestGenerateKeepsTheUserWhenPlayfabCannotBeOpened(t *testing.T) {
	store := &mapStore{values: map[string]models.StoredValue{
		"playfab": {Version: 99, Data: json.RawMessage(`{"DataVersion":99}`)},
		"auth":    {Data: json.RawMessage(`"2026-01-02T03:04:05Z"`)},
		"items":   {Data: json.RawMessage(`null`)},
		// avatarMetadata is stored as a string that itself holds JSON.
		"avatarMetadata": {Data: json.RawMessage(`"{\"v\":1,\"twr\":0,\"wlr\":0,\"ai\":1,\"ac\":0}"`)},
	}}
	persistentMap, err := models.NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	users := &Users{}
	users.Initialize()

	user := users.Generate("", persistentMap, nil, nil, "/steam/1", false, 1, "player")
	if user == nil {
		t.Fatal("the user must still be created when the playfab document cannot be opened")
	}
	athensUser, ok := user.(*User)
	if !ok {
		t.Fatalf("expected a *user.User, got %T", user)
	}
	if athensUser.PlayfabData != nil {
		t.Error("the unavailable playfab document should have been left nil")
	}
	if err = athensUser.PlayfabData.WithReadOnly(func(*Data) error { return nil }); !errors.Is(err, models.ErrDataUnavailable) {
		t.Errorf("using the unavailable document returned %v, want ErrDataUnavailable", err)
	}
	if athensUser.GetAlias() != "player" {
		t.Errorf("alias is %q, want player", athensUser.GetAlias())
	}
	if err = athensUser.GetAvatarMetadata().WithReadOnly(func(*string) error { return nil }); err != nil {
		t.Errorf("a healthy document stopped working: %s", err)
	}
	if store.values["playfab"].Version != 99 {
		t.Errorf("the future playfab document was rewritten: %+v", store.values["playfab"])
	}
}
