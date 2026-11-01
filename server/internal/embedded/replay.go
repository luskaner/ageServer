// Package embedded runs ageBattleServer inside the age server process and
// optionally stores the replay data it records.
package embedded

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luskaner/ageServer/common/paths"
	"github.com/luskaner/ageServer/server/internal/logger"
)

const (
	replayMagic         = "ALSBREPL"
	replayFormatVersion = 1
	replaySuffix        = ".replay"
	autoFolder          = "auto"

	// maxReplayHeaderSize bounds the JSON header (it embeds user-provided
	// server metadata) before it is prefixed with its length.
	maxReplayHeaderSize = 1 << 20
)

// ReplayMeta is the provenance written at the top of every replay file.
type ReplayMeta struct {
	GameID   string
	Region   string
	Name     string
	ServerID string
}

// ReplayOptions configures the replay store of one game. The values come from
// the [BattleServerReplays] section after validation in the command package.
type ReplayOptions struct {
	Enabled      bool
	Folder       string
	MaxFileSize  int64
	MaxTotalSize int64
	Expiry       time.Duration
}

// ReplayFolder is the default per-game folder when ReplayOptions.Folder is
// 'auto': <resources>/userData/<gameId>/replays.
func ReplayFolder(gameID string) string {
	return filepath.Join(paths.ResourcesDir, "userData", gameID, "replays")
}

// replayHeader is the JSON document stored after the fixed file header.
type replayHeader struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	Game      string `json:"game"`
	SessionID uint64 `json:"sessionId"`
	Started   string `json:"started"`
	Region    string `json:"region,omitempty"`
	Name      string `json:"name,omitempty"`
	ServerID  string `json:"serverId,omitempty"`
}

// replayFile is one .replay file. file is nil for files left by a previous run:
// they are only accounted for so the quota and the TTL cover them too.
type replayFile struct {
	file      *os.File
	name      string
	size      int64
	modTime   time.Time
	sessionID uint64
}

// ReplayStore writes the chunks ageBattleServer delivers per session to a
// self-describing .replay container. The format is:
//
//	"ALSBREPL" (8 bytes) | version (1 byte) | header length (u32 LE) | JSON header
//	then, repeated: timestamp (u64 LE, Unix milliseconds) | length (u32 LE) | data
//
// One file is kept per session. Session 0 is ignored: it means the sending
// client is not in a session yet, and the official replay host drops those too
// because they cannot be attributed to a match.
type ReplayStore struct {
	mu           sync.Mutex
	gameID       string
	folder       string
	root         *os.Root
	files        map[string]*replayFile
	sessions     map[uint64]string
	retired      map[uint64]struct{}
	total        int64
	maxFileSize  int64
	maxTotalSize int64
	expiry       time.Duration
	now          func() time.Time
	meta         ReplayMeta
	stop         chan struct{}
	stopped      chan struct{}
	closeOnce    sync.Once
}

// NewReplayStore prepares the folder, purges leftovers older than the TTL and
// starts the background expirer.
func NewReplayStore(gameID string, opts ReplayOptions, meta ReplayMeta) (*ReplayStore, error) {
	return newReplayStore(gameID, opts, meta, true)
}

func newReplayStore(gameID string, opts ReplayOptions, meta ReplayMeta, startSweeper bool) (*ReplayStore, error) {
	if !opts.Enabled {
		return nil, nil
	}
	if opts.MaxFileSize <= 0 || opts.MaxTotalSize <= 0 {
		return nil, errors.New("replay limits must be greater than zero")
	}
	// Expiry <= 0 means keep forever (no TTL-based expiration)
	base := strings.TrimSpace(opts.Folder)
	if base == "" || strings.EqualFold(base, autoFolder) {
		base = filepath.Join(paths.ResourcesDir, "userData")
	}
	folder := filepath.Join(base, gameID, "replays")
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create replay folder %q: %w", folder, err)
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil, fmt.Errorf("open replay folder %q: %w", folder, err)
	}
	store := &ReplayStore{
		gameID:       gameID,
		folder:       folder,
		root:         root,
		files:        make(map[string]*replayFile),
		sessions:     make(map[uint64]string),
		retired:      make(map[uint64]struct{}),
		maxFileSize:  opts.MaxFileSize,
		maxTotalSize: opts.MaxTotalSize,
		expiry:       opts.Expiry,
		now:          time.Now,
		meta:         meta,
	}
	if err := store.loadExisting(); err != nil {
		_ = root.Close()
		return nil, err
	}
	store.evictLocked(store.maxTotalSize, "")
	// Only start the sweeper if expiry is positive (TTL enabled)
	if startSweeper && opts.Expiry > 0 {
		store.stop = make(chan struct{})
		store.stopped = make(chan struct{})
		go store.sweep()
	}
	return store, nil
}

// Folder is the resolved per-game folder, for logging and diagnostics.
func (m *ReplayStore) Folder() string {
	if m == nil {
		return ""
	}
	return m.folder
}

// Append records one chunk for sessionID. It is called synchronously by
// ageBattleServer, so it only does one buffered file write per chunk.
func (m *ReplayStore) Append(sessionID uint64, data []byte) {
	if m == nil || sessionID == 0 || len(data) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == nil {
		return
	}
	if _, retired := m.retired[sessionID]; retired {
		if _, active := m.sessions[sessionID]; !active {
			return
		}
	}
	rf := m.sessionFileLocked(sessionID)
	if rf == nil {
		return
	}
	now := m.now()
	recordSize := int64(12 + len(data))
	if m.maxFileSize > 0 && rf.size+recordSize > m.maxFileSize {
		m.retireSessionLocked(sessionID, rf, "the maximum file size")
		return
	}
	if m.maxTotalSize > 0 && m.total+recordSize > m.maxTotalSize {
		m.evictLocked(m.maxTotalSize-recordSize, rf.name)
	}
	if m.maxTotalSize > 0 && m.total+recordSize > m.maxTotalSize {
		m.retireSessionLocked(sessionID, rf, "the total size limit")
		return
	}
	record := make([]byte, recordSize)
	binary.LittleEndian.PutUint64(record[0:8], uint64(now.UnixMilli()))
	binary.LittleEndian.PutUint32(record[8:12], uint32(len(data)))
	copy(record[12:], data)
	n, err := rf.file.Write(record)
	if err != nil {
		logger.Warn("Could not write battle server replay for session %d: %s", sessionID, err)
		m.retireSessionLocked(sessionID, rf, "a write error")
		return
	}
	rf.size += int64(n)
	m.total += int64(n)
	rf.modTime = now
	if n != len(record) {
		logger.Warn("Short write on battle server replay for session %d", sessionID)
		m.retireSessionLocked(sessionID, rf, "a short write")
	}
}

// sessionFileLocked returns the open file for a session, creating it with its
// header on the first chunk. Caller must hold m.mu.
func (m *ReplayStore) sessionFileLocked(sessionID uint64) *replayFile {
	if name, ok := m.sessions[sessionID]; ok {
		return m.files[name]
	}
	now := m.now()
	stamp := now.UTC().Format("20060102T150405.000000000Z")
	name := fmt.Sprintf("%s-%d%s", stamp, sessionID, replaySuffix)
	for i := 1; ; i++ {
		if _, exists := m.files[name]; !exists {
			break
		}
		name = fmt.Sprintf("%s-%d-%d%s", stamp, sessionID, i, replaySuffix)
	}
	file, err := m.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		logger.Warn("Could not create battle server replay for session %d: %s", sessionID, err)
		m.retired[sessionID] = struct{}{}
		return nil
	}
	header, err := m.headerBytes(sessionID, now)
	if err == nil {
		_, err = file.Write(header)
	}
	if err != nil {
		_ = file.Close()
		_ = m.root.Remove(name)
		logger.Warn("Could not write battle server replay header for session %d: %s", sessionID, err)
		m.retired[sessionID] = struct{}{}
		return nil
	}
	rf := &replayFile{
		file:      file,
		name:      name,
		size:      int64(len(header)),
		modTime:   now,
		sessionID: sessionID,
	}
	m.files[name] = rf
	m.sessions[sessionID] = name
	m.total += int64(len(header))
	return rf
}

func (m *ReplayStore) headerBytes(sessionID uint64, started time.Time) ([]byte, error) {
	header, err := json.Marshal(replayHeader{
		Format:    "ageServer-battle-server-replay",
		Version:   replayFormatVersion,
		Game:      m.meta.GameID,
		SessionID: sessionID,
		Started:   started.UTC().Format(time.RFC3339Nano),
		Region:    m.meta.Region,
		Name:      m.meta.Name,
		ServerID:  m.meta.ServerID,
	})
	if err != nil {
		return nil, err
	}
	if len(header) > maxReplayHeaderSize {
		return nil, fmt.Errorf("replay header too large: %d bytes", len(header))
	}
	// Build the frame with appends so the allocation size never relies on
	// adding up lengths that could overflow (the header contains user data).
	out := make([]byte, 0, len(replayMagic)+5)
	out = append(out, replayMagic...)
	out = append(out, replayFormatVersion)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(header)))
	out = append(out, header...)
	return out, nil
}

// retireSessionLocked stops recording a session permanently: the file is kept
// as a truncated but still parseable replay, and later chunks for the same
// session are ignored instead of creating a fresh file every time.
func (m *ReplayStore) retireSessionLocked(sessionID uint64, rf *replayFile, reason string) {
	if rf.file != nil {
		_ = rf.file.Close()
		rf.file = nil
	}
	if m.sessions[sessionID] == rf.name {
		delete(m.sessions, sessionID)
	}
	m.retired[sessionID] = struct{}{}
	logger.Warn("Stopped recording battle server replay for session %d in %s: %s was reached", sessionID, m.gameID, reason)
}

// loadExisting accounts for the files a previous run left behind and purges the
// ones whose TTL has already passed.
func (m *ReplayStore) loadExisting() error {
	entries, err := os.ReadDir(m.folder)
	if err != nil {
		return fmt.Errorf("read replay folder %q: %w", m.folder, err)
	}
	now := m.now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		if !info.Mode().IsRegular() {
			continue
		}
		if now.Sub(info.ModTime()) > m.expiry {
			_ = m.root.Remove(name)
			continue
		}
		m.files[name] = &replayFile{
			name:    name,
			size:    info.Size(),
			modTime: info.ModTime(),
		}
		m.total += info.Size()
	}
	return nil
}

// expire removes files that have not been written to for longer than the TTL.
func (m *ReplayStore) expire() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for _, rf := range m.files {
		if now.Sub(rf.modTime) > m.expiry {
			logger.Detail("Removed expired battle server replay %s", rf.name)
			m.removeLocked(rf)
		}
	}
}

// evictLocked deletes the oldest files until the accumulated size fits limit.
// keep is the name of the file that is about to be written and must not be
// evicted to make room for itself.
func (m *ReplayStore) evictLocked(limit int64, keep string) {
	if m.maxTotalSize <= 0 {
		return
	}
	for m.total > limit {
		var oldest *replayFile
		for _, rf := range m.files {
			if rf.name == keep {
				continue
			}
			if oldest == nil || rf.modTime.Before(oldest.modTime) {
				oldest = rf
			}
		}
		if oldest == nil {
			return
		}
		logger.Detail("Evicted battle server replay %s to stay under the total size limit", oldest.name)
		m.removeLocked(oldest)
	}
}

func (m *ReplayStore) removeLocked(rf *replayFile) {
	if rf.file != nil {
		_ = rf.file.Close()
		rf.file = nil
	}
	if err := m.root.Remove(rf.name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Detail("Could not delete battle server replay %s: %s", rf.name, err)
	}
	delete(m.files, rf.name)
	if rf.sessionID != 0 && m.sessions[rf.sessionID] == rf.name {
		delete(m.sessions, rf.sessionID)
		m.retired[rf.sessionID] = struct{}{}
	}
	m.total -= rf.size
	if m.total < 0 {
		m.total = 0
	}
}

func (m *ReplayStore) sweep() {
	defer close(m.stopped)
	interval := m.expiry / 4
	if interval < time.Second {
		interval = time.Second
	}
	if interval > time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.expire()
		}
	}
}

// Close stops the expirer and releases the open files. The replay payloads stay
// on disk: the TTL is what removes them, not a clean exit.
func (m *ReplayStore) Close() {
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
		for _, rf := range m.files {
			if rf.file != nil {
				_ = rf.file.Close()
				rf.file = nil
			}
		}
		if m.root != nil {
			_ = m.root.Close()
			m.root = nil
		}
		m.files = nil
		m.sessions = nil
		m.total = 0
	})
}
