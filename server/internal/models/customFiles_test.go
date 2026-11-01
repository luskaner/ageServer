package models

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func writeBytes(b []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	}
}

// newTestCustomFiles initializes an enabled instance rooted at t.TempDir and
// returns it plus the resolved folder (<base>/<gameId>).
func newTestCustomFiles(t *testing.T, gameId string, opts *CustomFilesOpts) (*MainCustomFiles, string) {
	t.Helper()
	base := t.TempDir()
	if opts == nil {
		opts = &CustomFilesOpts{}
	}
	opts.Enabled = true
	opts.Folder = base
	if opts.Expiry <= 0 {
		opts.Expiry = time.Hour
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = 1 << 20
	}
	if opts.MaxTotalSize <= 0 {
		opts.MaxTotalSize = 1 << 20
	}
	m := &MainCustomFiles{}
	m.Initialize(gameId, opts)
	if !m.Enabled() {
		t.Fatal("custom files should be enabled")
	}
	t.Cleanup(m.Close)
	return m, filepath.Join(base, gameId)
}

func TestCustomFilesPutOpen(t *testing.T) {
	m, _ := newTestCustomFiles(t, "age2", nil)
	data := []byte("hello savegame")
	cf, err := m.Put(123, "TEST2.aoe2mpgame", writeBytes(data))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if cf.Id != 123 || cf.Filename != "TEST2.aoe2mpgame" || cf.Length != len(data) {
		t.Fatalf("unexpected metadata: %+v", cf)
	}
	sum := md5.Sum(data)
	wantMD5 := hex.EncodeToString(sum[:])
	if cf.MD5 != wantMD5 {
		t.Fatalf("md5 = %s want %s", cf.MD5, wantMD5)
	}
	got, rc, ok := m.Open(123)
	if !ok {
		t.Fatal("Open returned not found")
	}
	defer rc.Close()
	read, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(read, data) {
		t.Fatalf("bytes = %q want %q", read, data)
	}
	if got.Length != len(data) {
		t.Fatalf("length = %d", got.Length)
	}
	if _, _, ok := m.Open(999); ok {
		t.Fatal("Open of unknown id should fail")
	}
}

func TestCustomFilesTTLExpires(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	base := t.TempDir()
	m := &MainCustomFiles{}
	m.Initialize("age2", &CustomFilesOpts{
		Enabled:      true,
		Folder:       base,
		MaxFileSize:  1 << 20,
		MaxTotalSize: 1 << 20,
		Expiry:       time.Hour,
		Now:          clock.now,
	})
	t.Cleanup(m.Close)

	if _, err := m.Put(7, "a.aoe2mpgame", writeBytes([]byte("payload"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, _, ok := m.Open(7); !ok {
		t.Fatal("file should exist before expiry")
	}
	onDisk := filepath.Join(base, "age2", "7-a.aoe2mpgame")
	if _, err := os.Stat(onDisk); err != nil {
		t.Fatalf("file should be on disk: %v", err)
	}

	clock.advance(2 * time.Hour)
	m.expire()

	if _, _, ok := m.Open(7); ok {
		t.Fatal("file should be expired")
	}
	if _, err := os.Stat(onDisk); !os.IsNotExist(err) {
		t.Fatalf("expired file still on disk: %v", err)
	}
	if m.Len() != 0 || m.TotalBytes() != 0 {
		t.Fatalf("index/bytes not cleaned: len=%d bytes=%d", m.Len(), m.TotalBytes())
	}
}

func TestCustomFilesStartupPurge(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "age2")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "1-old.aoe2mpgame")
	recent := filepath.Join(dir, "2-new.aoe2mpgame")
	part := filepath.Join(dir, "upload-abc.part")
	for _, p := range []string{old, recent, part} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	m := &MainCustomFiles{}
	m.Initialize("age2", &CustomFilesOpts{
		Enabled:      true,
		Folder:       base,
		MaxFileSize:  1 << 20,
		MaxTotalSize: 1 << 20,
		Expiry:       time.Hour,
	})
	t.Cleanup(m.Close)

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("stale file should be purged, stat err=%v", err)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Errorf("orphan .part should be purged, stat err=%v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("recent file should remain: %v", err)
	}
}

func TestCustomFilesReplaceSameId(t *testing.T) {
	m, dir := newTestCustomFiles(t, "age2", nil)
	if _, err := m.Put(5, "first.aoe2mpgame", writeBytes([]byte("first data"))); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Put(5, "second.aoe2mpgame", writeBytes([]byte("second"))); err != nil {
		t.Fatal(err)
	}
	if m.Len() != 1 {
		t.Fatalf("len = %d want 1", m.Len())
	}
	if m.TotalBytes() != int64(len("second")) {
		t.Fatalf("bytes = %d want %d", m.TotalBytes(), len("second"))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, e := range entries {
		if !e.IsDir() {
			files++
		}
	}
	if files != 1 {
		t.Fatalf("disk files = %d want 1 (%v)", files, entries)
	}
	_, rc, ok := m.Open(5)
	if !ok {
		t.Fatal("Open after replace")
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "second" {
		t.Fatalf("content = %q want second", got)
	}
}

func TestCustomFilesPathTraversal(t *testing.T) {
	m, dir := newTestCustomFiles(t, "age2", nil)
	cf, err := m.Put(9, "../../evil.txt", writeBytes([]byte("x")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if strings.Contains(cf.Path, "..") || strings.ContainsAny(cf.Path, `/\`) {
		t.Fatalf("unsafe stored path %q", cf.Path)
	}
	full := filepath.Join(dir, cf.Path)
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("file not inside root: %v", err)
	}
	if rel, err := filepath.Rel(dir, full); err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("file escaped root: %q", full)
	}
	// A filename that sanitizes to nothing falls back to <matchId>.bin.
	cf2, err := m.Put(10, "..", writeBytes([]byte("y")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if cf2.Filename != "10.bin" {
		t.Fatalf("fallback filename = %q want 10.bin", cf2.Filename)
	}
}

func TestCustomFilesWriteErrorCleansUp(t *testing.T) {
	m, dir := newTestCustomFiles(t, "age2", nil)
	boom := errors.New("boom")
	_, err := m.Put(1, "a.aoe2mpgame", func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v want boom", err)
	}
	if m.Len() != 0 || m.TotalBytes() != 0 {
		t.Fatalf("index not clean: len=%d bytes=%d", m.Len(), m.TotalBytes())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("leftover files on disk: %v", entries)
	}
}

func TestCustomFilesMaxFileSize(t *testing.T) {
	m, _ := newTestCustomFiles(t, "age2", &CustomFilesOpts{
		MaxFileSize:  4,
		MaxTotalSize: 100,
	})
	_, err := m.Put(1, "a.aoe2mpgame", writeBytes([]byte("12345")))
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v want ErrFileTooLarge", err)
	}
	if m.Len() != 0 || m.TotalBytes() != 0 {
		t.Fatalf("rejected upload was indexed")
	}
}

func TestCustomFilesEvictsOldest(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	base := t.TempDir()
	m := &MainCustomFiles{}
	m.Initialize("age2", &CustomFilesOpts{
		Enabled:      true,
		Folder:       base,
		MaxFileSize:  100,
		MaxTotalSize: 10,
		Expiry:       time.Hour,
		Now:          clock.now,
	})
	t.Cleanup(m.Close)

	for i := int32(1); i <= 3; i++ {
		if _, err := m.Put(i, "a.aoe2mpgame", writeBytes([]byte("1111"))); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
		clock.advance(time.Minute)
	}
	if m.Len() != 2 {
		t.Fatalf("len = %d want 2", m.Len())
	}
	if _, _, ok := m.Open(1); ok {
		t.Fatal("oldest file should have been evicted")
	}
	if _, _, ok := m.Open(3); !ok {
		t.Fatal("newest file should remain")
	}
	if m.TotalBytes() > 10 {
		t.Fatalf("bytes = %d over the limit", m.TotalBytes())
	}
}

func TestCustomFilesConcurrent(t *testing.T) {
	m, _ := newTestCustomFiles(t, "age2", nil)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		id := int32(i%5 + 1)
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, _ = m.Put(id, "a.aoe2mpgame", writeBytes([]byte("data")))
		}()
		go func() {
			defer wg.Done()
			if _, rc, ok := m.Open(id); ok {
				_, _ = io.Copy(io.Discard, rc)
				_ = rc.Close()
			}
		}()
		go func() {
			defer wg.Done()
			m.Delete(id)
		}()
	}
	wg.Wait()
}

// TestCustomFilesAdoptSameIdConcurrently pins the ordering Adopt has to keep:
// the file on disk and the metadata in the index must describe the same upload.
// Renaming outside the index lock let two uploads of one id publish metadata
// for a file whose bytes were no longer the ones on disk, which the download
// then served with a Content-Length it could not honour.
func TestCustomFilesAdoptSameIdConcurrently(t *testing.T) {
	for attempt := range 50 {
		m, dir := newTestCustomFiles(t, "age2", nil)
		long := make([]byte, 4096)
		short := []byte("BB")
		s1, err := m.Stage(writeBytes(long))
		if err != nil {
			t.Fatal(err)
		}
		s2, err := m.Stage(writeBytes(short))
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{}, 2)
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = m.Adopt(s1, 1, "a.aoe2mpgame", "")
		}()
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = m.Adopt(s2, 1, "a.aoe2mpgame", "")
		}()
		<-done
		<-done

		cf, rc, ok := m.Open(1)
		if !ok {
			t.Fatal("adopted file is missing from the index")
		}
		onDisk, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(onDisk) != cf.Length {
			t.Fatalf("attempt %d: index says %d bytes, disk holds %d", attempt, cf.Length, len(onDisk))
		}
		if m.TotalBytes() != int64(len(onDisk)) {
			t.Fatalf("attempt %d: accounted %d bytes, disk holds %d", attempt, m.TotalBytes(), len(onDisk))
		}
		if m.Len() != 1 {
			t.Fatalf("attempt %d: %d entries for one id", attempt, m.Len())
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("attempt %d: %d files on disk for one id: %v", attempt, len(entries), entries)
		}
		m.Close()
	}
}

// TestCustomFilesCloseKeepsFiles pins that a clean exit is not what deletes a
// payload. The launcher stops the server when the game exits, so wiping the
// folder here would make CustomFiles.Expiry meaningless and lose every pending
// savegame on each restart.
func TestCustomFilesCloseKeepsFiles(t *testing.T) {
	m, dir := newTestCustomFiles(t, "age2", nil)
	cf, err := m.Put(3, "a.aoe2mpgame", writeBytes([]byte("payload")))
	if err != nil {
		t.Fatal(err)
	}
	onDisk := filepath.Join(dir, cf.Path)
	m.Close()

	if _, err := os.Stat(onDisk); err != nil {
		t.Fatalf("Close removed the payload: %v", err)
	}
	if m.Enabled() {
		t.Fatal("Close should leave the instance disabled")
	}
	if m.Len() != 0 || m.TotalBytes() != 0 {
		t.Fatalf("Close should drop the index: len=%d bytes=%d", m.Len(), m.TotalBytes())
	}
	// A later startup purges it once it is older than the TTL.
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(onDisk, past, past); err != nil {
		t.Fatal(err)
	}
	restarted := &MainCustomFiles{}
	restarted.Initialize("age2", &CustomFilesOpts{
		Enabled: true, Folder: filepath.Dir(dir),
		MaxFileSize: 1 << 20, MaxTotalSize: 1 << 20, Expiry: time.Hour,
	})
	t.Cleanup(restarted.Close)
	if _, err := os.Stat(onDisk); !os.IsNotExist(err) {
		t.Fatalf("startup should have purged the expired leftover, stat err=%v", err)
	}
}

func TestKnownCustomFileExtension(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"TEST2.aoe2mpgame", true},
		{"replay.AOE2RPL", true},
		{"noextension", true},
		{"save.txt", false},
		{"archive.zip", false},
	}
	for _, tc := range cases {
		if got := KnownCustomFileExtension(tc.name); got != tc.want {
			t.Errorf("KnownCustomFileExtension(%q) = %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestCustomFilesOpenSnapshotDuringOverwrite(t *testing.T) {
	m, _ := newTestCustomFiles(t, "age2", nil)
	payloads := [][]byte{[]byte("first"), bytes.Repeat([]byte("other"), 100)}
	if _, err := m.Put(7, "same.aoe2mpgame", writeBytes(payloads[0])); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for n := 0; n < 100; n++ {
			if _, err := m.Put(7, "same.aoe2mpgame", writeBytes(payloads[n%2])); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for n := 0; n < 100; n++ {
		cf, rc, ok := m.Open(7)
		if !ok {
			t.Error("indexed file missing during overwrite")
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		sum := md5.Sum(data)
		if err != nil || len(data) != cf.Length || hex.EncodeToString(sum[:]) != cf.MD5 {
			t.Errorf("metadata and payload from different uploads: length=%d metadata=%+v err=%v", len(data), cf, err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
