package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/luskaner/ageServer/server/internal/logger"
)

// StoredValue is a single persisted document: the schema version it was
// written with plus its opaque payload.
//
// The payload is never interpreted below the PersistentJsonData layer, which is
// what lets the same game code sit on top of files, SQLite or anything else.
type StoredValue struct {
	Version uint32
	Data    json.RawMessage
}

// UserStore is the durable storage seam behind one user's data.
//
// Implementations only provide Load and Put. Caching, per-key locking, schema
// upgrades and the on-the-wire JSON encoding all live above this interface, so
// a new backend is a single small type with no knowledge of the game's data
// model.
//
// Load is called once per user, when that user is first opened. A user with
// nothing stored yet must return an empty map and a nil error rather than an
// error. An unreadable single key should be dropped so the player keeps access
// to the rest of the account, but a store that cannot be opened at all must
// return an error.
//
// Put must be atomic: once it returns, an interruption must leave either the
// previous document or the new one, never a truncated mix of the two. Keys are
// independent, so implementations are not required to make two Puts atomic
// with respect to each other.
//
// Once Load has returned, a UserStore is not read again: every read is served
// from the in-memory cache, so an implementation never has to serve reads and
// writes for the same document at the same time.
type UserStore interface {
	Load() (map[string]StoredValue, error)
	Put(key string, value StoredValue) error
}

// UserStoreFactory opens the store backing one user.
//
// Replacing this function is the only change needed to run the server on a real
// database: it is the single place where the storage backend is chosen.
type UserStoreFactory func(gameId string, isXbox bool, platformUserId uint64) (UserStore, error)

// userDataSuffix is the extension of both the per-key documents and the
// single-file saves written before the per-key layout existed.
const userDataSuffix = ".json"

// jsonUserStore keeps one JSON document per key inside dir.
//
// Storing a key per file is what makes a single change cheap: updating an
// avatar stat rewrites a few hundred bytes instead of the whole save, which for
// an Age of Empires IV player means a few hundred bytes instead of 277 KB of
// untouched inventory. It also bounds the damage of an interrupted write to the
// one key being written.
type jsonUserStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSONUserStore returns a UserStore backed by one JSON file per key, rooted
// at dir. The directory is created on demand.
func NewJSONUserStore(dir string) UserStore {
	return &jsonUserStore{dir: dir}
}

// JSONUserStoreFactory is the default UserStoreFactory: per-key JSON files under
// the user's directory in resources/userData.
//
// It never fails today, but keeps the same signature a real database factory
// needs so that swapping the backend does not touch its callers.
func JSONUserStoreFactory(gameId string, isXbox bool, platformUserId uint64) (UserStore, error) {
	return NewJSONUserStore(UserDataDir(gameId, !isXbox, strconv.FormatUint(platformUserId, 10))), nil
}

func (s *jsonUserStore) keyPath(key string) string {
	// The keys currently in use are all alphanumeric, so escaping leaves the
	// file names readable while still making any key safe as a file name.
	return filepath.Join(s.dir, url.PathEscape(key)+userDataSuffix)
}

func (s *jsonUserStore) Load() (values map[string]StoredValue, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.migrateLegacy(); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(s.dir, os.ModePerm); err != nil {
		return nil, err
	}
	var entries []fs.DirEntry
	if entries, err = os.ReadDir(s.dir); err != nil {
		return nil, err
	}
	values = make(map[string]StoredValue, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), userDataSuffix) {
			continue
		}
		var key string
		var unescapeErr error
		if key, unescapeErr = url.PathUnescape(strings.TrimSuffix(entry.Name(), userDataSuffix)); unescapeErr != nil {
			logger.Warn("Ignoring user data file with an undecodable name in %s: %s", s.dir, entry.Name())
			continue
		}
		value, readErr := s.readKey(key)
		if readErr != nil {
			// One damaged key must not lock the player out of the rest of the
			// account. The key is kept aside for inspection and the game
			// rebuilds it from its default.
			logger.Warn("User data key %q in %s is unreadable (%s); moving it aside and resetting it to defaults", key, s.dir, readErr)
			s.quarantine(key)
			continue
		}
		values[key] = value
	}
	return values, nil
}

func (s *jsonUserStore) Put(key string, value StoredValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.put(key, value)
}

// put is Put without the lock, for callers that already hold it.
func (s *jsonUserStore) put(key string, value StoredValue) error {
	if err := os.MkdirAll(s.dir, os.ModePerm); err != nil {
		return err
	}
	contents, err := json.Marshal(jsonDataWithMetadata[json.RawMessage]{
		Metadata: jsonMetadata{Version: value.Version},
		Data:     value.Data,
	})
	if err != nil {
		return err
	}
	return writeFileAtomic(s.keyPath(key), contents)
}

func (s *jsonUserStore) readKey(key string) (StoredValue, error) {
	contents, err := os.ReadFile(s.keyPath(key))
	if err != nil {
		return StoredValue{}, err
	}
	var decoded jsonDataWithMetadata[json.RawMessage]
	if err = json.Unmarshal(contents, &decoded); err != nil {
		return StoredValue{}, err
	}
	if decoded.Data == nil {
		// Valid JSON that carries no document, e.g. a truncated write that
		// happened to end on a brace. Treating it as absent lets the key be
		// rebuilt instead of locking the player out.
		return StoredValue{}, errors.New("no document in file")
	}
	return StoredValue{Version: decoded.Metadata.Version, Data: decoded.Data}, nil
}

func (s *jsonUserStore) quarantine(key string) {
	path := s.keyPath(key)
	if err := os.Rename(path, path+".corrupt"); err != nil {
		logger.Warn("Could not set the damaged user data key %q aside: %s", key, err)
	}
}

// migrateLegacy converts a single-file save into one file per key.
//
// The old file is only renamed away once every key has been written, so an
// interrupted migration simply runs again on the next start and rewrites the
// same values: the conversion is idempotent.
//
// A legacy file that cannot be parsed is set aside rather than reported: a
// player must not be locked out of the account, and this is the one place where
// losing the file means losing everything, so it is kept for inspection instead
// of being deleted.
func (s *jsonUserStore) migrateLegacy() error {
	legacyPath := s.dir + userDataSuffix
	contents, err := os.ReadFile(legacyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var legacy jsonDataWithMetadata[map[string]jsonDataWithMetadata[json.RawMessage]]
	if err = json.Unmarshal(contents, &legacy); err != nil {
		logger.Warn("Legacy user data %s could not be read (%s); moving it aside and starting this user fresh", legacyPath, err)
		return os.Rename(legacyPath, legacyPath+".corrupt")
	}
	logger.Info("Converting legacy user data %s to the per-key layout", legacyPath)
	for key, value := range legacy.Data {
		if err = s.put(key, StoredValue{Version: value.Metadata.Version, Data: value.Data}); err != nil {
			return fmt.Errorf("converting legacy user data %s: %w", legacyPath, err)
		}
	}
	if err = os.Rename(legacyPath, legacyPath+".migrated"); err != nil {
		return fmt.Errorf("retiring legacy user data %s: %w", legacyPath, err)
	}
	return nil
}

// writeFileAtomic replaces path with contents so that a reader only ever sees a
// complete document.
//
// The bytes are written to a temporary file in the same directory, flushed to
// disk and only then renamed over the target. The previous implementation
// truncated the live file and streamed into it, so any interruption - a crash,
// a full disk or a power cut during a LAN game - left the player's save
// truncated and unparseable, permanently locking them out of their account.
//
// The parent directory is deliberately not synced: losing the rename itself
// only leaves the previous complete document behind, which is harmless.
//
// Note that on Windows the rename is refused while any other handle to the
// target is open, because Go opens files without FILE_SHARE_DELETE. That is
// safe, just not silent, and it cannot happen in practice: a user's documents
// are read once when the user is opened and only written afterwards.
func writeFileAtomic(path string, contents []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		// A no-op once the rename below has succeeded.
		_ = os.Remove(tmp.Name())
	}()
	if _, err = tmp.Write(contents); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	// CreateTemp uses 0600; keep the permissions saves have always had so the
	// files stay readable when the server runs under a different account.
	if err = os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
