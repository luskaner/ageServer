package models

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

// ErrFileTooLarge is returned by CustomFiles.Put / Stage when the stream
// exceeds the configured per-file limit.
var ErrFileTooLarge = errors.New("custom file exceeds the maximum size")

const (
	autoFolder           = "auto"
	customFilesSubfolder = "custom-files"
	partPrefix           = "upload-"
	partSuffix           = ".part"
	fallbackSuffix       = ".bin"
	// removeAttempts and removeRetryDelay bound how long a delete keeps trying
	// while an in-flight download holds the file open.
	removeAttempts   = 3
	removeRetryDelay = 50 * time.Millisecond
)

// customFileExtensions is the allowlist of extensions accepted for the
// filename sent by the client. An empty extension is allowed: the file is then
// stored under the fallback name <matchId>.bin.
var customFileExtensions = map[string]bool{
	"aoe2mpgame":   true,
	"mpgame":       true,
	"aoe2rpl":      true,
	"aoe3rpl":      true,
	"aoe4rpl":      true,
	"aomrpl":       true,
	"aoe1rpl":      true,
	"rpl":          true,
	"aoe2scenario": true,
	"aoe3scenario": true,
	"aoe4scenario": true,
	"aomscenario":  true,
	"aoe1scenario": true,
	"scenario":     true,
	"scn":          true,
	"savegame":     true,
	"campaign":     true,
}

// Folder is the default per-game base used when CustomFiles.Folder is 'auto':
// <system temp>/ageServer/custom-files/<gameId>.
func Folder(gameId string) string {
	return filepath.Join(os.TempDir(), common.Name, customFilesSubfolder, gameId)
}

// CustomFile is the metadata of one stored custom file. The payload itself
// lives on disk; the index only keeps the bookkeeping needed to serve and
// expire it.
type CustomFile struct {
	Id       int32
	Filename string
	// OriginalFilename is the basename advertised to the game. Filename and
	// Path remain sanitized for disk storage; stripping Unicode from the
	// advertised name would make the client look for a different savegame.
	OriginalFilename string
	ContentType      string
	// Path is relative to the CustomFiles root.
	Path    string
	MD5     string
	CRC32   string
	Length  int
	Created time.Time
	expiry  time.Time
}

// Expiry is when the file becomes eligible for deletion by the sweeper.
func (f *CustomFile) Expiry() time.Time {
	return f.expiry
}

// CustomFileStage is a fully written but not yet indexed file. It is created by
// Stage (before the matchId is known) and turned into a CustomFile by Adopt.
type CustomFileStage struct {
	path   string
	md5    string
	crc32  string
	length int
	done   bool
}

// Length is the number of bytes streamed into the stage.
func (s *CustomFileStage) Length() int {
	return s.length
}

// CustomFilesOpts configures one game's CustomFiles instance. A nil pointer
// passed to Initialize falls back to the values published in the internal
// runtime globals (i.e. the parsed config).
type CustomFilesOpts struct {
	Enabled      bool
	Folder       string
	MaxFileSize  int64
	MaxTotalSize int64
	Expiry       time.Duration
	// Now overrides the clock. Tests use it to drive expiry deterministically.
	Now func() time.Time
}

// DefaultCustomFilesOpts returns the options resolved from the global runtime
// configuration. Used by CreateMainGame when a game does not supply its own.
func DefaultCustomFilesOpts() *CustomFilesOpts {
	return &CustomFilesOpts{
		Enabled:      i.CustomFilesEnabled,
		Folder:       i.CustomFilesFolder,
		MaxFileSize:  i.CustomFilesMaxFileSize,
		MaxTotalSize: i.CustomFilesMaxTotalSize,
		Expiry:       i.CustomFilesExpiry,
	}
}

type CustomFiles interface {
	Initialize(gameId string, opts *CustomFilesOpts)
	Enabled() bool
	MaxFileSize() int64
	Put(id int32, filename string, write func(w io.Writer) error) (*CustomFile, error)
	Stage(write func(w io.Writer) error) (*CustomFileStage, error)
	Adopt(stage *CustomFileStage, id int32, filename string, contentType string) (*CustomFile, error)
	Discard(stage *CustomFileStage)
	Open(id int32) (*CustomFile, io.ReadCloser, bool)
	Delete(id int32)
	Len() int
	TotalBytes() int64
	Close()
}

// MainCustomFiles is the disk-backed implementation of CustomFiles. Each game
// gets its own instance with its own root and sweeper, so no gameId branching
// is ever needed.
type MainCustomFiles struct {
	mu           sync.Mutex
	files        *i.SafeMap[int32, *CustomFile]
	root         *os.Root
	folder       string
	totalBytes   int64
	expiry       time.Duration
	maxFileSize  int64
	maxTotalSize int64
	now          func() time.Time
	enabled      bool
	stop         chan struct{}
	// stopped is closed by the sweeper on its way out, so Close can wait for
	// the goroutine to be gone before dropping the state it reads.
	stopped   chan struct{}
	closeOnce sync.Once
}

func (m *MainCustomFiles) Enabled() bool {
	return m.enabled && m.root != nil
}

func (m *MainCustomFiles) MaxFileSize() int64 {
	return m.maxFileSize
}

func (m *MainCustomFiles) TotalBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalBytes
}

func (m *MainCustomFiles) Initialize(gameId string, opts *CustomFilesOpts) {
	m.files = i.NewSafeMap[int32, *CustomFile]()
	m.now = time.Now
	if opts == nil {
		opts = DefaultCustomFilesOpts()
	}
	if opts.Now != nil {
		m.now = opts.Now
	}
	m.maxFileSize = opts.MaxFileSize
	m.maxTotalSize = opts.MaxTotalSize
	m.expiry = opts.Expiry
	if !opts.Enabled {
		return
	}
	base := opts.Folder
	if base == "" || base == autoFolder {
		base = filepath.Join(os.TempDir(), common.Name, customFilesSubfolder)
	}
	folder := filepath.Join(base, gameId)
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		logger.Warn("Custom files disabled for %s: cannot create %s: %s", gameId, folder, err)
		return
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		logger.Warn("Custom files disabled for %s: cannot open %s: %s", gameId, folder, err)
		return
	}
	m.folder = folder
	m.root = root
	m.enabled = true
	m.purgeStale()
	m.stop = make(chan struct{})
	m.stopped = make(chan struct{})
	go m.sweep()
}

// Stage streams write into a provisional file and returns it. The file is not
// visible to Open until Adopt is called; Discard removes it. maxFileSize is
// enforced while streaming and MD5 is computed in the same pass.
func (m *MainCustomFiles) Stage(write func(w io.Writer) error) (*CustomFileStage, error) {
	if !m.Enabled() {
		return nil, fs.ErrNotExist
	}
	tmp, err := os.CreateTemp(m.folder, partPrefix+"*"+partSuffix)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(tmp.Name())
	hash := md5.New()
	crc := crc32.NewIEEE()
	counter := &limitedWriter{limit: m.maxFileSize}
	writeErr := write(io.MultiWriter(hash, crc, counter, tmp))
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = m.root.Remove(name)
		if errors.Is(writeErr, ErrFileTooLarge) {
			return nil, ErrFileTooLarge
		}
		return nil, writeErr
	}
	return &CustomFileStage{
		path:   name,
		md5:    hex.EncodeToString(hash.Sum(nil)),
		crc32:  fmt.Sprintf("%08x", crc.Sum32()),
		length: int(counter.n),
	}, nil
}

// Adopt moves a staged file under its final name, indexes it and accounts for
// its size. The previous file with the same id, if any, is replaced.
func (m *MainCustomFiles) Adopt(stage *CustomFileStage, id int32, filename string, contentType string) (*CustomFile, error) {
	if !m.Enabled() || stage == nil {
		return nil, fs.ErrNotExist
	}
	originalFilename := filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = sanitizeFilename(filename)
	if filename == "" {
		filename = strconv.FormatInt(int64(id), 10) + fallbackSuffix
	}
	if originalFilename == "" || originalFilename == "." || originalFilename == ".." {
		originalFilename = filename
	}
	finalName := strconv.FormatInt(int64(id), 10) + "-" + filename
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	cf := &CustomFile{
		Id:               id,
		Filename:         filename,
		OriginalFilename: originalFilename,
		ContentType:      contentType,
		Path:             finalName,
		MD5:              stage.md5,
		CRC32:            stage.crc32,
		Length:           stage.length,
	}
	// The rename belongs under the same lock as the index update. Two uploads
	// racing on one id both write the same final name, so renaming outside the
	// lock lets the index be published for a file whose bytes are no longer the
	// ones on disk, and the download then announces a Content-Length it cannot
	// honour.
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.root.Rename(stage.path, finalName); err != nil {
		return nil, err
	}
	stage.done = true
	now := m.now().UTC()
	cf.Created = now
	cf.expiry = now.Add(m.expiry)
	if old, ok := m.files.Load(id); ok {
		// Rename already replaced the target when the name is unchanged; only
		// remove the old payload when it lives under a different name.
		if old.Path != finalName {
			m.removeFile(old.Path)
		}
		m.totalBytes -= int64(old.Length)
	}
	m.files.Store(id, cf, nil)
	m.totalBytes += int64(cf.Length)
	m.evictLocked()
	return cf, nil
}

// Discard removes a staged file that was never adopted. It is a no-op once the
// stage has been adopted or already discarded.
func (m *MainCustomFiles) Discard(stage *CustomFileStage) {
	if stage == nil || stage.done || !m.Enabled() {
		return
	}
	stage.done = true
	m.removeFile(stage.path)
}

func (m *MainCustomFiles) Put(id int32, filename string, write func(w io.Writer) error) (*CustomFile, error) {
	stage, err := m.Stage(write)
	if err != nil {
		return nil, err
	}
	cf, err := m.Adopt(stage, id, filename, "")
	if err != nil {
		m.Discard(stage)
		return nil, err
	}
	return cf, nil
}

// Open returns the metadata and a streaming reader for one file. The caller
// owns the reader and must close it.
func (m *MainCustomFiles) Open(id int32) (*CustomFile, io.ReadCloser, bool) {
	// Keep the metadata and opened payload from the same upload generation.
	// Adopt can otherwise replace the path between Load and Open.
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Enabled() {
		return nil, nil, false
	}
	cf, ok := m.files.Load(id)
	if !ok {
		return nil, nil, false
	}
	f, err := m.root.Open(cf.Path)
	if err != nil {
		return nil, nil, false
	}
	return cf, f, true
}

func (m *MainCustomFiles) Delete(id int32) {
	if !m.Enabled() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cf, ok := m.files.Load(id); ok {
		m.deleteLocked(cf)
	}
}

func (m *MainCustomFiles) Len() int {
	if m.files == nil {
		return 0
	}
	return m.files.Len()
}

// Close stops the sweeper and releases the root.
//
// It deliberately leaves the files on disk: the TTL is what removes them, not a
// clean exit. Wiping the folder here would make CustomFiles.Expiry meaningless
// for the ordinary path - the launcher stops the server when the game exits, so
// every restart would take the pending savegames with it. What survives is
// unreachable, because the index starts empty and is not persisted; a later
// startup finds those files older than the TTL and purgeStale removes them.
func (m *MainCustomFiles) Close() {
	m.closeOnce.Do(func() {
		if m.stop != nil {
			close(m.stop)
			// The sweeper reads the index and the root without the lock, so it
			// has to be gone before the state it touches is dropped.
			<-m.stopped
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.root != nil {
			_ = m.root.Close()
		}
		m.root = nil
		m.enabled = false
		m.files = i.NewSafeMap[int32, *CustomFile]()
		m.totalBytes = 0
	})
}

// removeFile deletes name from the root, retrying a few times. Windows refuses
// to delete a file that a Download still holds open, and without the retry the
// payload would stay on disk with no index entry left that could ever reach it.
func (m *MainCustomFiles) removeFile(name string) {
	var err error
	for attempt := range removeAttempts {
		if err = m.root.Remove(name); err == nil {
			return
		}
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		if attempt < removeAttempts-1 {
			time.Sleep(removeRetryDelay)
		}
	}
	logger.Detail("Could not delete custom file %s: %s", name, err)
}

func (m *MainCustomFiles) deleteLocked(cf *CustomFile) {
	m.removeFile(cf.Path)
	m.files.Delete(cf.Id)
	m.totalBytes -= int64(cf.Length)
}

// evictLocked deletes the oldest entries until the accumulated size fits
// maxTotalSize again. Caller must hold m.mu.
func (m *MainCustomFiles) evictLocked() {
	if m.maxTotalSize <= 0 {
		return
	}
	for m.totalBytes > m.maxTotalSize && m.files.Len() > 0 {
		oldest := m.oldestLocked()
		if oldest == nil {
			return
		}
		logger.Detail("Evicting custom file %d (%s) to stay under the total size limit", oldest.Id, oldest.Filename)
		m.deleteLocked(oldest)
	}
}

func (m *MainCustomFiles) oldestLocked() *CustomFile {
	var oldest *CustomFile
	for cf := range m.files.Values() {
		if oldest == nil || cf.Created.Before(oldest.Created) {
			oldest = cf
		}
	}
	return oldest
}

// purgeStale removes leftovers from previous runs: every provisional .part file
// and every regular file older than the TTL.
func (m *MainCustomFiles) purgeStale() {
	entries, err := os.ReadDir(m.folder)
	if err != nil {
		return
	}
	now := m.now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, partPrefix) && strings.HasSuffix(name, partSuffix) {
			_ = m.root.Remove(name)
			continue
		}
		if m.expiry <= 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > m.expiry {
			_ = m.root.Remove(name)
		}
	}
}

func (m *MainCustomFiles) sweep() {
	defer close(m.stopped)
	for {
		timer := time.NewTimer(m.nextExpiration())
		select {
		case <-m.stop:
			timer.Stop()
			return
		case <-timer.C:
			m.expire()
		}
	}
}

// expire deletes every entry whose TTL has passed. Exposed as a method (rather
// than inlined in sweep) so tests with an injected clock can drive it directly.
func (m *MainCustomFiles) expire() {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for cf := range m.files.Values() {
		if cf.expiry.Before(now) || cf.expiry.Equal(now) {
			logger.Detail("Removed expired custom file %d (%s)", cf.Id, cf.Filename)
			m.deleteLocked(cf)
		}
	}
}

func (m *MainCustomFiles) nextExpiration() time.Duration {
	var next time.Time
	for cf := range m.files.Values() {
		if next.IsZero() || cf.expiry.Before(next) {
			next = cf.expiry
		}
	}
	const minInterval = 100 * time.Millisecond
	if next.IsZero() {
		if m.expiry > minInterval {
			return m.expiry
		}
		return minInterval
	}
	d := next.Sub(m.now())
	if d < minInterval {
		d = minInterval
	}
	return d
}

// limitedWriter fails as soon as a write would push the total past limit.
type limitedWriter struct {
	limit int64
	n     int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.limit <= 0 || lw.n+int64(len(p)) > lw.limit {
		return 0, ErrFileTooLarge
	}
	lw.n += int64(len(p))
	return len(p), nil
}

var unsafeFilenameChars = strings.NewReplacer(
	"/", "_", "\\", "_",
)

// sanitizeFilename reduces a client-supplied filename to a single safe path
// component built only from [A-Za-z0-9._-]. It returns "" when nothing usable
// is left, in which case callers fall back to <matchId>.bin.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = unsafeFilenameChars.Replace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	name = strings.Trim(b.String(), ".")
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return name
}

// KnownCustomFileExtension reports whether the client-provided filename carries
// an extension this server accepts. An empty extension is allowed because the
// model then stores it under <matchId>.bin.
func KnownCustomFileExtension(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" {
		return true
	}
	return customFileExtensions[ext]
}
