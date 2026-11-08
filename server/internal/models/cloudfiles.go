package models

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"iter"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

const (
	builtinSubfolder = "builtin"
	replaysSubfolder = "replays"

	// CloudBuiltinPrefix is the index prefix of the assets shipped with the
	// server. The cloud routes look assets up with it.
	CloudBuiltinPrefix = builtinSubfolder + "/"

	// partTTL is how long an unfinished upload may stay on disk before the
	// maintenance sweep deletes it.
	partTTL = time.Hour

	cloudfilesVersion = "2019-02-02"
	cloudfilesType    = "application/octet-stream"
)

// ErrNotReplay rejects writes and deletes outside the replays subfolder, so a
// leaked signature can never overwrite a distributed asset.
var ErrNotReplay = errors.New("cloud file key is not a replay path")

// ErrReplaysDisabled is returned when the [Replays] section or the cloud store
// itself is unavailable.
var ErrReplaysDisabled = errors.New("replays are disabled")

type CloudfilesIndex struct {
	Id       int    `json:"id"`
	Key      string `json:"key"`
	Type     string `json:"type"`
	Version  string `json:"version"`
	ETag     string `json:"etag"`
	Created  string `json:"created"`
	Length   int
	Checksum string
}

// CloudFilesReplayOpts configures the replay side of the store. The zero value
// disables uploads and keeps every replay forever.
type CloudFilesReplayOpts struct {
	Enabled      bool
	MaxFileSize  int64
	MaxTotalSize int64
	Expiry       time.Duration
	// Now overrides the clock. Tests use it to drive expiry deterministically.
	Now func() time.Time
}

// CloudFiles is the single cloud store of one game: the static assets shipped
// under builtin/ plus the replays uploaded by the game under replays/. Value
// and Credentials stay exported because the cloud routes read them from
// another package.
type CloudFiles struct {
	mu          sync.RWMutex
	root        *os.Root
	Value       map[string]CloudfilesIndex
	Credentials Credentials
	replayOpts  CloudFilesReplayOpts
	stop        chan struct{}
	stopped     chan struct{}
	closeOnce   sync.Once
}

// Enabled reports whether the store has an open root and can serve bytes.
func (m *CloudFiles) Enabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root != nil
}

// ReplaysEnabled reports whether the [Replays] section allows uploads.
func (m *CloudFiles) ReplaysEnabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.replayOpts.Enabled
}

// MaxFileSize is the per-replay upload cap, or 0 when uploads are unlimited.
// The upload handler needs it to size the connection read deadline.
func (m *CloudFiles) MaxFileSize() int64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.replayOpts.MaxFileSize
}

func (m *CloudFiles) now() time.Time {
	if m.replayOpts.Now != nil {
		return m.replayOpts.Now()
	}
	return time.Now()
}

// SetReplayOpts publishes the replay limits, purges expired leftovers and
// starts the maintenance goroutine. It runs after BuildCloudfilesIndex because
// only then is the configuration known.
func (m *CloudFiles) SetReplayOpts(opts CloudFilesReplayOpts) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.replayOpts = opts
	hasRoot := m.root != nil
	startSweeper := hasRoot && m.stop == nil
	if startSweeper {
		m.stop = make(chan struct{})
		m.stopped = make(chan struct{})
	}
	m.mu.Unlock()
	if !hasRoot {
		return
	}
	if startSweeper {
		go m.sweep()
	}
	if opts.Enabled {
		m.purgeStale()
	}
	m.purgeParts()
}

// Get returns the index entry for an exact key.
func (m *CloudFiles) Get(key string) (*CloudfilesIndex, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.Value[key]
	if !ok {
		return nil, false
	}
	return &entry, true
}

// Values iterates the index under the read lock. Callers cannot hold the map
// themselves: Put and Delete mutate it while requests iterate it.
func (m *CloudFiles) Values() iter.Seq[CloudfilesIndex] {
	return func(yield func(CloudfilesIndex) bool) {
		if m == nil {
			return
		}
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, entry := range m.Value {
			if !yield(entry) {
				return
			}
		}
	}
}

// GetByKey finds an entry by its URL key, which is the storage path.
func (m *CloudFiles) GetByKey(key string) (string, *CloudfilesIndex, bool) {
	if m == nil {
		return "", nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for filename, file := range m.Value {
		if file.Key == key {
			return filename, &file, true
		}
	}
	return "", nil, false
}

func (m *CloudFiles) ReadFile(name string) ([]byte, error) {
	if m == nil {
		return nil, fs.ErrNotExist
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.root == nil {
		return nil, fs.ErrNotExist
	}
	if _, ok := m.Value[name]; !ok {
		return nil, fs.ErrNotExist
	}
	return m.root.ReadFile(name)
}

// Open returns the metadata and a streaming reader for key, so downloads do
// not load the whole replay into memory and can serve ranges.
func (m *CloudFiles) Open(key string) (*CloudfilesIndex, io.ReadCloser, bool) {
	if m == nil {
		return nil, nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.root == nil {
		return nil, nil, false
	}
	entry, ok := m.Value[key]
	if !ok {
		return nil, nil, false
	}
	file, err := m.root.Open(key)
	if err != nil {
		return nil, nil, false
	}
	return &entry, file, true
}

// Credential issues an upload credential for key, reusing the shared
// Credentials store. It does not reserve anything: the handler validates the
// signature before Put writes.
func (m *CloudFiles) Credential(key string) (string, time.Time, bool) {
	if m == nil {
		return "", time.Time{}, false
	}
	key = path.Clean(key)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.root == nil || !m.replayOpts.Enabled || m.Credentials == nil {
		return "", time.Time{}, false
	}
	cred := CreateCredential(m.Credentials, &key)
	return cred.Id(), cred.Expiry(), true
}

// FindReplay finds the stored replay of a match. With the deterministic key
// used by the upload flow this is equivalent to Get(replayKey(matchID, kind));
// it still works for files uploaded under another naming scheme.
func (m *CloudFiles) FindReplay(matchID int64, kind int) (string, *CloudfilesIndex, bool) {
	if m == nil {
		return "", nil, false
	}
	prefix := fmt.Sprintf("%s/%d/%d/", replaysSubfolder, matchID, kind)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, entry := range m.Value {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".gz") {
			return key, &entry, true
		}
	}
	return "", nil, false
}

// Put streams write into a unique temporary file next to its destination,
// computes length and MD5 in the same pass, renames it into place atomically
// and publishes the index entry.
func (m *CloudFiles) Put(key string, write func(io.Writer) error) (*CloudfilesIndex, error) {
	key = path.Clean(key)
	if !isReplayPath(key) {
		return nil, ErrNotReplay
	}
	m.mu.RLock()
	root := m.root
	enabled := m.replayOpts.Enabled
	maxFileSize := m.replayOpts.MaxFileSize
	now := m.now()
	m.mu.RUnlock()
	if root == nil || !enabled {
		return nil, ErrReplaysDisabled
	}
	if err := root.MkdirAll(path.Dir(key), os.ModePerm); err != nil {
		return nil, err
	}

	// The suffix is random hexadecimal, not the credential id: credentials are
	// base64 and may contain '/', which would create a nested folder.
	part := key + ".part-" + randomHex(8)
	file, err := root.Create(part)
	if err != nil {
		return nil, err
	}
	renamed := false
	defer func() {
		_ = file.Close()
		if !renamed {
			_ = root.Remove(part)
		}
	}()

	hasher := md5.New()
	length := &countWriter{}
	writers := []io.Writer{hasher, length}
	if maxFileSize > 0 {
		writers = append(writers, &limitedWriter{limit: maxFileSize})
	}
	writers = append(writers, file)
	writeErr := write(io.MultiWriter(writers...))
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		if errors.Is(writeErr, ErrFileTooLarge) {
			return nil, ErrFileTooLarge
		}
		return nil, writeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil || !m.replayOpts.Enabled {
		return nil, ErrReplaysDisabled
	}
	if err := m.root.Rename(part, key); err != nil {
		return nil, err
	}
	renamed = true
	entry := CloudfilesIndex{
		Key:      key,
		Type:     cloudfilesType,
		Version:  cloudfilesVersion,
		ETag:     `"0x` + checksumHex(hasher) + `"`,
		Created:  now.UTC().Format(http.TimeFormat),
		Length:   int(length.n),
		Checksum: base64.StdEncoding.EncodeToString(hasher.Sum(nil)),
	}
	m.Value[key] = entry
	m.evictLocked()
	return &entry, nil
}

// Delete removes a replay and drops its index entry. The parent folder is
// removed when it becomes empty; ENOTEMPTY (the sibling artifact of age4) is
// expected and ignored.
func (m *CloudFiles) Delete(key string) {
	key = path.Clean(key)
	if !isReplayPath(key) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil {
		return
	}
	m.removeLocked(key)
}

// Close stops the maintenance goroutine and releases the root. Uploaded files
// stay on disk: the TTL is what removes them.
func (m *CloudFiles) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		if m.stop != nil {
			close(m.stop)
			<-m.stopped
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.root != nil {
			_ = m.root.Close()
		}
		m.root = nil
	})
}

// BuildCloudfilesIndex prepares both subfolders, loads the static index and
// rebuilds the replay index from disk. It never returns nil: a store without
// builtin assets is still valid for replays and always carries credentials.
func BuildCloudfilesIndex(configFolder string, baseFolder string) *CloudFiles {
	index := &CloudFiles{
		Value:       map[string]CloudfilesIndex{},
		Credentials: NewCredentials(),
	}
	if baseFolder == "" {
		logger.Warn("Cloud files disabled: empty base folder")
		return index
	}
	if err := os.MkdirAll(filepath.Join(baseFolder, builtinSubfolder), os.ModePerm); err != nil {
		logger.Warn("Cloud files disabled for %s: cannot create %s: %s", baseFolder, builtinSubfolder, err)
		return index
	}
	if err := os.MkdirAll(filepath.Join(baseFolder, replaysSubfolder), os.ModePerm); err != nil {
		logger.Warn("Replays disabled for %s: cannot create %s: %s", baseFolder, replaysSubfolder, err)
		return index
	}
	root, err := os.OpenRoot(baseFolder)
	if err != nil {
		logger.Warn("Cloud files disabled for %s: cannot open %s: %s", baseFolder, baseFolder, err)
		return index
	}
	index.root = root

	index.loadStatic(configFolder)
	index.loadExisting()
	return index
}

// loadStatic resolves every name listed in cloudfilesIndex.json against
// builtin/. The JSON keys stay bare filenames; the published index is keyed by
// the storage path so ReadFile/Open can use it directly.
func (m *CloudFiles) loadStatic(configFolder string) {
	data, err := os.ReadFile(filepath.Join(configFolder, "cloudfilesIndex.json"))
	if err != nil {
		// Normal in age1/age4/athens: no static assets, but replays work.
		return
	}
	var listed map[string]CloudfilesIndex
	if err := json.Unmarshal(data, &listed); err != nil {
		logger.Warn("Invalid cloudfilesIndex.json for %s: %s", configFolder, err)
		return
	}
	resolved := make(map[string]CloudfilesIndex, len(listed))
	for name, fileInfo := range listed {
		storagePath := builtinSubfolder + "/" + name
		content, err := m.root.ReadFile(storagePath)
		if err != nil {
			// Missing assets used to panic. A broken index must not take the
			// server down: log it and keep serving replays.
			logger.Warn("Cloud file %s listed in the index is missing: %s", storagePath, err)
			continue
		}
		fileInfo.Length = len(content)
		sum := md5.Sum(content)
		fileInfo.Checksum = base64.StdEncoding.EncodeToString(sum[:])
		resolved[storagePath] = fileInfo
	}
	m.Value = resolved
}

// loadExisting rebuilds the replay index from disk. Temporary uploads found on
// the way are deleted instead of indexed.
func (m *CloudFiles) loadExisting() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil {
		return
	}
	fsys := m.root.FS()
	_ = fs.WalkDir(fsys, replaysSubfolder, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		base := path.Base(name)
		if strings.Contains(base, ".part-") {
			_ = m.root.Remove(name)
			return nil
		}
		if !strings.HasSuffix(base, ".gz") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if _, exists := m.Value[name]; exists {
			return nil
		}
		m.Value[name] = CloudfilesIndex{
			Key:     name,
			Type:    cloudfilesType,
			Version: cloudfilesVersion,
			Created: info.ModTime().UTC().Format(http.TimeFormat),
			Length:  int(info.Size()),
		}
		return nil
	})
}

// removeLocked deletes a replay and its index entry. Caller must hold m.mu.
func (m *CloudFiles) removeLocked(key string) {
	if err := m.root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Detail("Could not delete cloud replay %s: %s", key, err)
	}
	delete(m.Value, key)
	parent := path.Dir(key)
	for parent != replaysSubfolder && parent != "." {
		// Remove fails with ENOTEMPTY when the sibling artifact is still there.
		if err := m.root.Remove(parent); err != nil {
			break
		}
		parent = path.Dir(parent)
	}
}

// purgeStale removes expired replays and the empty folders left behind.
func (m *CloudFiles) purgeStale() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil || m.replayOpts.Expiry <= 0 {
		return
	}
	now := m.now()
	fsys := m.root.FS()
	var dirs []string
	_ = fs.WalkDir(fsys, replaysSubfolder, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if name != replaysSubfolder {
				dirs = append(dirs, name)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if now.Sub(info.ModTime()) > m.replayOpts.Expiry {
			logger.Detail("Removed expired cloud replay %s", name)
			m.removeLocked(name)
		}
		return nil
	})
	for idx := len(dirs) - 1; idx >= 0; idx-- {
		_ = m.root.Remove(dirs[idx])
	}
}

// purgeParts deletes upload temporaries older than partTTL. It runs even when
// replays never expire: a killed process can leave a .part-<rand> behind.
func (m *CloudFiles) purgeParts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil {
		return
	}
	now := m.now()
	fsys := m.root.FS()
	_ = fs.WalkDir(fsys, replaysSubfolder, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if !strings.Contains(path.Base(name), ".part-") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if now.Sub(info.ModTime()) > partTTL {
			_ = m.root.Remove(name)
		}
		return nil
	})
}

// expire removes every replay whose TTL has passed.
func (m *CloudFiles) expire() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil || m.replayOpts.Expiry <= 0 {
		return
	}
	now := m.now()
	for key := range m.Value {
		if !strings.HasPrefix(key, replaysSubfolder+"/") {
			continue
		}
		entry := m.Value[key]
		created, err := time.Parse(http.TimeFormat, entry.Created)
		if err != nil || now.Sub(created) <= m.replayOpts.Expiry {
			continue
		}
		logger.Detail("Removed expired cloud replay %s", key)
		m.removeLocked(key)
	}
}

// evictLocked deletes the oldest replays until the accumulated size fits the
// configured total. Assets under builtin/ are never candidates. Caller must
// hold m.mu.
func (m *CloudFiles) evictLocked() {
	limit := m.replayOpts.MaxTotalSize
	if m.root == nil || limit <= 0 {
		return
	}
	total := int64(0)
	for key, entry := range m.Value {
		if strings.HasPrefix(key, replaysSubfolder+"/") {
			total += int64(entry.Length)
		}
	}
	for total > limit {
		var oldestKey string
		var oldestTime time.Time
		for key, entry := range m.Value {
			if !strings.HasPrefix(key, replaysSubfolder+"/") {
				continue
			}
			created, err := time.Parse(http.TimeFormat, entry.Created)
			if err != nil {
				created = time.Time{}
			}
			if oldestKey == "" || created.Before(oldestTime) {
				oldestKey, oldestTime = key, created
			}
		}
		if oldestKey == "" {
			return
		}
		logger.Detail("Evicted cloud replay %s to stay under the total size limit", oldestKey)
		total -= int64(m.Value[oldestKey].Length)
		m.removeLocked(oldestKey)
	}
}

func (m *CloudFiles) sweep() {
	defer close(m.stopped)
	ticker := time.NewTicker(m.sweepInterval())
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.expire()
			m.purgeParts()
		}
	}
}

func (m *CloudFiles) sweepInterval() time.Duration {
	m.mu.RLock()
	expiry := m.replayOpts.Expiry
	m.mu.RUnlock()
	if expiry <= 0 {
		return partTTL / 4
	}
	interval := expiry / 4
	if interval < time.Second {
		interval = time.Second
	}
	if interval > time.Minute {
		interval = time.Minute
	}
	return interval
}

// isReplayPath accepts only clean paths under replays/. path.Clean (not
// filepath.Clean: keys always use '/') collapses replays/a/../b to replays/b
// before the checks.
//
// A backslash is rejected outright: it is a separator for os.Root on Windows
// but not for path.Clean, so "replays/x\..\..\builtin/_data.txt" would pass the
// prefix check and then resolve to builtin/_data.txt. Replay keys never contain
// one.
func isReplayPath(name string) bool {
	if strings.ContainsRune(name, '\\') {
		return false
	}
	name = path.Clean(name)
	if !strings.HasPrefix(name, replaysSubfolder+"/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "" {
			return false
		}
	}
	return true
}

func randomHex(n int) string {
	b := make([]byte, n)
	i.WithRng(func(rand *i.RandReader) {
		for idx := range b {
			b[idx] = byte(rand.UintN(256))
		}
	})
	return hex.EncodeToString(b)
}

func checksumHex(hasher hash.Hash) string {
	return hex.EncodeToString(hasher.Sum(nil)[:16])
}

// countWriter records how many bytes went through it, so Put can publish the
// length without stat-ing the file after it has been closed.
type countWriter struct {
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
