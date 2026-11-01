package models

import (
	"testing"
)

// isolateUserData points userDataFolder at a temporary directory for the
// duration of the test.
//
// userDataFolder is relative to the working directory, which during a test is
// the package directory, so without this the tests below create and migrate
// saves inside the repository's own resources folder.
func isolateUserData(t *testing.T) {
	t.Helper()
	original := userDataFolder
	userDataFolder = t.TempDir()
	t.Cleanup(func() { userDataFolder = original })
}

// Regression: when GenerateFn returned nil (e.g. due to a persistent data
// failure), LoadOrStoreFn cached that nil permanently. Every future request
// for the same identifier would return nil until server restart.
// The fix removes the cached entry so subsequent calls can retry.
func TestGetOrCreateUserNilGenerationDoesNotCachePermanently(t *testing.T) {
	isolateUserData(t)
	users := &MainUsers{}
	users.Initialize()
	users.GenerateFn = func(_ string, _ *PersistentStringJsonMap, _ Items, _ AvatarStatDefinitions, _ string, _ bool, _ uint64, _ string) User {
		return nil // simulate generation failure
	}

	const gameId = "age2"
	const identifier = "test-identifier"

	// First call: generation fails, returns nil.
	result1 := users.GetOrCreateUser(gameId, nil, nil, "", identifier, false, 0, "")
	if result1 != nil {
		t.Fatal("first call with failing GenerateFn should return nil")
	}

	// Verify the store no longer has the entry (cleanup ran).
	_, exists := users.store.Load(identifier)
	if exists {
		t.Fatal("nil user should have been evicted from the store")
	}

	// Fix GenerateFn to succeed; second call must retry and get a real user.
	callCount := 0
	users.GenerateFn = func(_ string, _ *PersistentStringJsonMap, _ Items, _ AvatarStatDefinitions, _ string, _ bool, _ uint64, _ string) User {
		callCount++
		return &MainUser{id: int32(100 + callCount), alias: "retry-test"}
	}

	result2 := users.GetOrCreateUser(gameId, nil, nil, "", identifier, false, 0, "")
	if result2 == nil {
		t.Fatal("second call after fixing GenerateFn should succeed")
	}
	if result2.GetId() != 101 {
		t.Fatalf("second call returned id %d, want 101", result2.GetId())
	}
}

func TestGetOrCreateUserCachesSuccessfulGeneration(t *testing.T) {
	isolateUserData(t)
	users := &MainUsers{}
	users.Initialize()
	callCount := 0
	users.GenerateFn = func(_ string, _ *PersistentStringJsonMap, _ Items, _ AvatarStatDefinitions, _ string, _ bool, _ uint64, _ string) User {
		callCount++
		return &MainUser{id: int32(callCount * 10), alias: "cached-user"}
	}

	const gameId = "age2"
	const id = "cache-success-id"

	r1 := users.GetOrCreateUser(gameId, nil, nil, "", id, false, 0, "")
	r2 := users.GetOrCreateUser(gameId, nil, nil, "", id, false, 0, "")

	if r1 == nil || r2 == nil {
		t.Fatal("both calls should return non-nil users")
	}
	if r1 != r2 {
		t.Fatal("second call must return the cached instance, not a new one")
	}
	if callCount != 1 {
		t.Fatalf("GenerateFn called %d times, want 1 (must serve from cache)", callCount)
	}
}
