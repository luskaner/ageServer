package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

// UpgradableData migrates a stored document to the version the server expects.
//
// UpgradeToNextVersion is called once per version between the stored one and
// CurrentVersion, so the first call receives the object ObjectOfVersion built
// for the stored version and every later call receives what the previous one
// returned.
type UpgradableData[T any] interface {
	ObjectOfVersion(version uint32) any
	CurrentVersion() uint32
	UpgradeToNextVersion(oldVersion uint32, oldObject any) T
}

type InitialUpgradableData[T any] struct {
}

func (i *InitialUpgradableData[T]) ObjectOfVersion(_ uint32) any {
	panic("should not have been called")
}

func (i *InitialUpgradableData[T]) CurrentVersion() uint32 {
	return 0
}

func (i *InitialUpgradableData[T]) UpgradeToNextVersion(_ uint32, _ any) T {
	panic("should not have been called")
}

type DefaultUpgradableData[T any] interface {
	UpgradableData[T]
	Default() T
}

type InitialUpgradableDefaultData[T any] struct {
	InitialUpgradableData[T]
}

// upgradeStored brings a stored document up to the upgrader's current version.
//
// The stored version is trusted to be lower than the current one: callers must
// reject a document that is newer before calling this, otherwise the loop
// count underflows.
func upgradeStored[T any](stored StoredValue, upgrader UpgradableData[T]) (data T, upgraded bool, err error) {
	versionsToUpgrade := upgrader.CurrentVersion() - stored.Version
	if versionsToUpgrade == 0 {
		return data, false, nil
	}
	currentData := upgrader.ObjectOfVersion(stored.Version)
	if err = json.Unmarshal(stored.Data, &currentData); err != nil {
		return data, false, err
	}
	for versionsUpgraded := range versionsToUpgrade {
		currentData = upgrader.UpgradeToNextVersion(stored.Version+versionsUpgraded, currentData)
	}
	return currentData.(T), true, nil
}

type jsonMetadata struct {
	Version uint32 `json:"version"`
}

type jsonDataWithMetadata[T any] struct {
	Metadata jsonMetadata `json:"metadata"`
	Data     T            `json:"data"`
}

type jsonAnyDataWithMetadata struct {
	Metadata jsonMetadata `json:"metadata"`
	Data     any          `json:"data"`
}

type PersistentStringJsonMapDataMap = internal.SafeMap[string, jsonAnyDataWithMetadata]

// PersistentStringJsonMap keeps a user's documents decoded and ready to read,
// backed by a UserStore for durability.
//
// Reads and writes are served from memory; the store is only touched when the
// user is opened and when a document actually changes. A write that cannot be
// persisted leaves the updated value in memory (so the session is not
// interrupted) and marks the key as dirty; the next write to the same key or
// a Flush() call will retry the persistence.
type PersistentStringJsonMap struct {
	store UserStore
	// cachedRawData mirrors what is on disk, keeping each document's schema
	// version alongside its bytes so upgrades can be applied on open.
	cachedRawData   *internal.SafeMap[string, StoredValue]
	currentData     *PersistentStringJsonMapDataMap
	currentDataLock *internal.KeyRWMutex[string]
	// dirty tracks keys whose in-memory value has diverged from disk and needs
	// to be retried. It is updated under the per-key lock that guards the
	// corresponding key in currentDataLock.
	dirty sync.Map
}

// NewPersistentStringMap opens the user's documents through store.
//
// The store format itself carries no version: each document carries its own,
// and is validated against the game data that reads it. A save written by a
// newer server is therefore still rejected key by key, and downgrading simply
// starts from an empty save that the newer server can read back.
func NewPersistentStringMap(store UserStore) (persistentMap *PersistentStringJsonMap, err error) {
	var stored map[string]StoredValue
	if stored, err = store.Load(); err != nil {
		return nil, err
	}
	cachedRawData := internal.NewSafeMap[string, StoredValue]()
	for key, value := range stored {
		cachedRawData.Store(key, value, nil)
	}
	return &PersistentStringJsonMap{
		store:           store,
		cachedRawData:   cachedRawData,
		currentData:     internal.NewSafeMap[string, jsonAnyDataWithMetadata](),
		currentDataLock: internal.NewKeyRWMutex[string](),
	}, nil
}

// persist writes one document to the store and then mirrors it in the cache.
//
// On success the key is marked clean. On failure the key is marked dirty, the
// in-memory caches are updated so the session sees the new value, and the
// error is returned so the caller can log it. The next write to the same key
// or a Flush() call will retry the persistence.
func (p *PersistentStringJsonMap) persist(key string, version uint32, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	stored := StoredValue{Version: version, Data: encoded}
	if err = p.store.Put(key, stored); err != nil {
		p.dirty.Store(key, struct{}{})
		// Update in-memory caches so the session sees the new value even
		// though it could not be persisted.
		p.cachedRawData.Store(key, stored, nil)
		return fmt.Errorf("persist %q: %w", key, err)
	}
	p.dirty.Delete(key)
	p.cachedRawData.Store(key, stored, nil)
	return nil
}

// Flush retries persisting all dirty keys. It is intended to be called at
// server shutdown to give pending writes a final chance to reach disk. It
// returns the last error encountered, if any, so the caller can log it.
func (p *PersistentStringJsonMap) Flush() error {
	var lastErr error
	p.dirty.Range(func(key, _ any) bool {
		keyStr := key.(string)
		stored, ok := p.cachedRawData.Load(keyStr)
		if !ok {
			// The key might have been deleted (not used in this server),
			// but if it's dirty we should still clean up.
			p.dirty.Delete(keyStr)
			return true
		}
		if err := p.store.Put(keyStr, stored); err != nil {
			lastErr = fmt.Errorf("flush %q: %w", keyStr, err)
			return true // continue to try other keys
		}
		p.dirty.Delete(keyStr)
		return true
	})
	return lastErr
}

// IsDirty reports whether the given key has uncommitted changes.
func (p *PersistentStringJsonMap) IsDirty(key string) bool {
	_, ok := p.dirty.Load(key)
	return ok
}

// DirtyKeys returns a slice of keys that have uncommitted changes.
func (p *PersistentStringJsonMap) DirtyKeys() []string {
	var keys []string
	p.dirty.Range(func(key, _ any) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

func (p *PersistentStringJsonMap) psmjSet[T any](key string, value DefaultUpgradableData[T]) (err error) {
	p.currentDataLock.Lock(key)
	defer p.currentDataLock.Unlock(key)
	var data T
	var needsSave bool
	stored, exists := p.cachedRawData.Load(key)
	switch {
	case !exists:
		data = value.Default()
		needsSave = true
	case stored.Version > value.CurrentVersion():
		return fmt.Errorf("user data key %q has version %d, newer than the supported version %d", key, stored.Version, value.CurrentVersion())
	default:
		upgradedData, upgraded, upgradeErr := upgradeStored(stored, value)
		switch {
		case upgradeErr != nil:
			return upgradeErr
		case upgraded:
			data = upgradedData
			needsSave = true
		default:
			err = json.Unmarshal(stored.Data, &data)
		}
	}
	if err != nil {
		return err
	}
	if _, exists := p.currentData.Store(key, jsonAnyDataWithMetadata{
		Metadata: jsonMetadata{Version: value.CurrentVersion()},
		Data:     data,
	}, func(_ jsonAnyDataWithMetadata) bool {
		return false
	}); exists {
		return fmt.Errorf("user data key %q is already in use", key)
	}
	if !needsSave {
		return nil
	}
	return p.persist(key, value.CurrentVersion(), data)
}

// psmjFn runs fn against the decoded document for key. T is always a pointer
// type, so fn mutates the document that is already held in currentData.
func (p *PersistentStringJsonMap) psmjFn[T any](key string, fn func(data T) error) (fullData jsonAnyDataWithMetadata, err error) {
	var ok bool
	if fullData, ok = p.currentData.Load(key); !ok {
		err = fmt.Errorf("user data key %q has not been loaded", key)
		return
	}
	typed, ok := fullData.Data.(T)
	if !ok {
		err = fmt.Errorf("user data key %q holds %T, not %T", key, fullData.Data, typed)
		return
	}
	err = fn(typed)
	return
}

func (p *PersistentStringJsonMap) withReadOnly[T any](key string, fn func(data T) error) error {
	p.currentDataLock.RLock(key)
	defer p.currentDataLock.RUnlock(key)
	_, err := p.psmjFn(key, fn)
	return err
}

func (p *PersistentStringJsonMap) withReadWrite[T any](key string, fn func(data T) error) error {
	p.currentDataLock.Lock(key)
	defer p.currentDataLock.Unlock(key)
	fullData, err := p.psmjFn(key, fn)
	if err != nil {
		return err
	}
	stored, exists := p.cachedRawData.Load(key)
	if !exists {
		// psmjSet caches a key before it becomes readable, so this can only
		// mean the two caches have fallen out of sync.
		return fmt.Errorf("user data key %q is not cached", key)
	}
	return p.persist(key, stored.Version, fullData.Data)
}

// ErrDataUnavailable is returned by a document that could not be opened, most
// often because its stored version is newer than this server understands.
//
// The handle still exists so the user can log in and use everything else; only
// the affected document reports this.
var ErrDataUnavailable = errors.New("user data is not available")

type PersistentJsonData[T any] struct {
	persistentMap *PersistentStringJsonMap
	key           string
}

// available reports whether this handle can be used, naming the document when
// it cannot. It is safe to call on a nil handle.
func (p *PersistentJsonData[T]) available() error {
	if p == nil {
		return fmt.Errorf("%w: unknown key", ErrDataUnavailable)
	}
	if p.persistentMap == nil {
		return fmt.Errorf("%w: %s", ErrDataUnavailable, p.key)
	}
	return nil
}

func (p *PersistentJsonData[T]) WithReadOnly(fn func(data T) error) error {
	if err := p.available(); err != nil {
		return err
	}
	return p.persistentMap.withReadOnly(p.key, fn)
}

func (p *PersistentJsonData[T]) WithReadWrite(fn func(data T) error) error {
	if err := p.available(); err != nil {
		return err
	}
	return p.persistentMap.withReadWrite(p.key, fn)
}

func (p *PersistentJsonData[T]) MarshalJSON() (data []byte, err error) {
	if err = p.available(); err != nil {
		return nil, err
	}
	err = p.WithReadOnly(func(d T) error {
		data, err = json.Marshal(d)
		return err
	})
	return
}

// OpenPersistentJsonData attaches a saved document to a user, reporting a
// failure to read or migrate it instead of discarding it.
//
// The returned handle is nil when the document could not be opened. That is
// deliberate: the user still logs in and every other document keeps working,
// while this one reports ErrDataUnavailable. Keeping the nil visible here is
// what makes the problem findable, since the routes cannot log it themselves.
func OpenPersistentJsonData[T any](persistentData *PersistentStringJsonMap, identifier string, key string, upgrader DefaultUpgradableData[T]) *PersistentJsonData[T] {
	data, err := NewPersistentJsonData(persistentData, key, upgrader)
	if err != nil {
		logger.Fail("User %s: %q could not be opened and will stay unavailable (%s)", identifier, key, err)
	}
	return data
}

func NewPersistentJsonData[T any](persistentMap *PersistentStringJsonMap, key string, upgrader DefaultUpgradableData[T]) (data *PersistentJsonData[T], err error) {
	if err = persistentMap.psmjSet(key, upgrader); err != nil {
		return
	}
	data = &PersistentJsonData[T]{
		persistentMap: persistentMap,
		key:           key,
	}
	return
}
