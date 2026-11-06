package models

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
)

func newTestCloudStore(t *testing.T) (*CloudFiles, string) {
	t.Helper()
	i.InitializeRng(42)
	base := t.TempDir()
	config := t.TempDir()
	store := BuildCloudfilesIndex(config, base)
	store.SetReplayOpts(CloudFilesReplayOpts{
		Enabled:      true,
		MaxFileSize:  1 << 20,
		MaxTotalSize: 4 << 20,
		Expiry:       -1,
	})
	t.Cleanup(store.Close)
	return store, base
}

func writeReplay(store *CloudFiles, key, content string) (*CloudfilesIndex, error) {
	return store.Put(key, func(w io.Writer) error {
		_, err := io.WriteString(w, content)
		return err
	})
}

func TestBuildCloudfilesIndexWithoutIndexIsStillEnabled(t *testing.T) {
	base := t.TempDir()
	store := BuildCloudfilesIndex(t.TempDir(), base)
	t.Cleanup(store.Close)

	if store == nil {
		t.Fatal("BuildCloudfilesIndex returned nil")
	}
	if !store.Enabled() {
		t.Fatal("a store without builtin assets must still be enabled for replays")
	}
	if store.Credentials == nil {
		t.Fatal("credentials must always be initialized")
	}
	for _, subfolder := range []string{builtinSubfolder, replaysSubfolder} {
		if info, err := os.Stat(filepath.Join(base, subfolder)); err != nil || !info.IsDir() {
			t.Fatalf("%s was not created: %v", subfolder, err)
		}
	}
	if len(store.Value) != 0 {
		t.Fatalf("Value = %v, want empty", store.Value)
	}
}

func TestBuildCloudfilesIndexResolvesBuiltinAssets(t *testing.T) {
	config := t.TempDir()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, builtinSubfolder), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, builtinSubfolder, "_data.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := `{"_data.txt":{"id":7,"key":"game/abc","type":"text/plain","version":"2019-02-02"}}`
	if err := os.WriteFile(filepath.Join(config, "cloudfilesIndex.json"), []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}
	store := BuildCloudfilesIndex(config, base)
	t.Cleanup(store.Close)

	entry, ok := store.Get("builtin/_data.txt")
	if !ok {
		t.Fatal("builtin asset was not indexed under its storage path")
	}
	if entry.Key != "game/abc" || entry.Length != len("asset") || entry.Checksum == "" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	data, err := store.ReadFile("builtin/_data.txt")
	if err != nil || string(data) != "asset" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	// A missing file listed by the index must warn, not panic.
	if err := os.Remove(filepath.Join(base, builtinSubfolder, "_data.txt")); err != nil {
		t.Fatal(err)
	}
	BuildCloudfilesIndex(config, base).Close()
}

func TestPutPublishesAndFindReplayFinds(t *testing.T) {
	store, base := newTestCloudStore(t)
	key := "replays/42/0/abc.gz"
	entry, err := writeReplay(store, key, "replay-bytes")
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || entry.Key != key || entry.Length != len("replay-bytes") {
		t.Fatalf("entry = %+v", entry)
	}
	if _, err := os.Stat(filepath.Join(base, key)); err != nil {
		t.Fatalf("replay was not written: %v", err)
	}
	foundKey, foundEntry, ok := store.FindReplay(42, 0)
	if !ok || foundKey != key || foundEntry.Length != entry.Length {
		t.Fatalf("FindReplay = %q, %+v, %t", foundKey, foundEntry, ok)
	}
	if _, _, ok := store.FindReplay(42, 1); ok {
		t.Fatal("FindReplay found a kind that was never uploaded")
	}
	file, reader, ok := store.Open(key)
	if !ok {
		t.Fatal("Open failed for a published replay")
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil || string(content) != "replay-bytes" || file.Length != len(content) {
		t.Fatalf("Open = %+v, %q, %v", file, content, err)
	}
}

func TestPutRejectsNonReplayKeysAndKeepsAssets(t *testing.T) {
	config := t.TempDir()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, builtinSubfolder), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, builtinSubfolder, "_data.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := `{"_data.txt":{"id":1,"key":"game/abc"}}`
	if err := os.WriteFile(filepath.Join(config, "cloudfilesIndex.json"), []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}
	store := BuildCloudfilesIndex(config, base)
	store.SetReplayOpts(CloudFilesReplayOpts{Enabled: true, MaxFileSize: 100, MaxTotalSize: 1000})
	t.Cleanup(store.Close)

	for _, key := range []string{
		"builtin/_data.txt",
		"replays/../../builtin/_data.txt",
	} {
		if _, err := writeReplay(store, key, "overwrite"); !errors.Is(err, ErrNotReplay) {
			t.Fatalf("Put(%q) = %v, want ErrNotReplay", key, err)
		}
	}
	if data, err := store.ReadFile("builtin/_data.txt"); err != nil || string(data) != "asset" {
		t.Fatalf("asset was modified: %q, %v", data, err)
	}
	store.Delete("builtin/_data.txt")
	if _, ok := store.Get("builtin/_data.txt"); !ok {
		t.Fatal("Delete removed a builtin asset")
	}
}

func TestIsReplayPath(t *testing.T) {
	for _, name := range []string{"replays/1/0/ab.gz", "replays/abc/1/x.gz"} {
		if !isReplayPath(name) {
			t.Fatalf("isReplayPath(%q) = false, want true", name)
		}
	}
	for _, name := range []string{
		"replays/../../builtin/_data.txt",
		"replays/../builtin/_data.txt",
		"builtin/_data.txt",
		"replays/",
		"replays",
		"/replays/1/0/ab.gz",
		// A backslash is a separator for os.Root on Windows but not for
		// path.Clean, so this would otherwise resolve to builtin/_data.txt.
		`replays/x\..\..\builtin/_data.txt`,
		`replays\..\..\builtin\_data.txt`,
	} {
		if isReplayPath(name) {
			t.Fatalf("isReplayPath(%q) = true, want false", name)
		}
	}
}

func TestPutEnforcesMaxFileSize(t *testing.T) {
	store, _ := newTestCloudStore(t)
	store.SetReplayOpts(CloudFilesReplayOpts{Enabled: true, MaxFileSize: 4, MaxTotalSize: 100})
	_, err := writeReplay(store, "replays/1/0/big.gz", "too large")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Put = %v, want ErrFileTooLarge", err)
	}
	if _, ok := store.Get("replays/1/0/big.gz"); ok {
		t.Fatal("a rejected upload was published")
	}
}

func TestDeleteRemovesEmptyParentAndKeepsSibling(t *testing.T) {
	store, base := newTestCloudStore(t)
	if _, err := writeReplay(store, "replays/7/0/a.gz", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := writeReplay(store, "replays/7/1/b.gz", "b"); err != nil {
		t.Fatal(err)
	}
	store.Delete("replays/7/0/a.gz")
	if _, err := os.Stat(filepath.Join(base, "replays/7/0")); !os.IsNotExist(err) {
		t.Fatalf("empty kind folder was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "replays/7/1/b.gz")); err != nil {
		t.Fatalf("sibling artifact was removed: %v", err)
	}
	store.Delete("replays/7/1/b.gz")
	if _, err := os.Stat(filepath.Join(base, "replays/7")); !os.IsNotExist(err) {
		t.Fatalf("empty match folder was not removed: %v", err)
	}
}

func TestCredentialDoesNotCreateFolders(t *testing.T) {
	store, base := newTestCloudStore(t)
	if _, _, ok := store.Credential("replays/9/0/x.gz"); !ok {
		t.Fatal("Credential failed on an enabled store")
	}
	entries, err := os.ReadDir(filepath.Join(base, replaysSubfolder))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Credential created %d entries under replays/", len(entries))
	}
}

func TestLoadExistingRebuildsIndexAndPurgesParts(t *testing.T) {
	base := t.TempDir()
	store := BuildCloudfilesIndex(t.TempDir(), base)
	store.SetReplayOpts(CloudFilesReplayOpts{Enabled: true, Expiry: -1})
	t.Cleanup(store.Close)
	folder := filepath.Join(base, "replays/11/0")
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "abc.gz"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "abc.gz.part-deadbeef"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	reopened := BuildCloudfilesIndex(t.TempDir(), base)
	t.Cleanup(reopened.Close)
	entry, ok := reopened.Get("replays/11/0/abc.gz")
	if !ok || entry.Length != len("hello") || entry.Checksum != "" {
		t.Fatalf("reopened entry = %+v, %t", entry, ok)
	}
	if _, ok := reopened.Get("replays/11/0/abc.gz.part-deadbeef"); ok {
		t.Fatal("a partial upload was indexed")
	}
	if _, err := os.Stat(filepath.Join(folder, "abc.gz.part-deadbeef")); !os.IsNotExist(err) {
		t.Fatalf("orphan part was not removed: %v", err)
	}
}

func TestPurgePartsUsesInjectedClock(t *testing.T) {
	store, base := newTestCloudStore(t)
	store.SetReplayOpts(CloudFilesReplayOpts{
		Enabled: true,
		Expiry:  -1,
		Now:     func() time.Time { return time.Now().Add(2 * partTTL) },
	})
	folder := filepath.Join(base, "replays/12/0")
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(folder, "x.gz.part-1234")
	if err := os.WriteFile(part, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	store.purgeParts()
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatalf("stale part was not purged: %v", err)
	}
}

func TestEvictionNeverTouchesBuiltin(t *testing.T) {
	config := t.TempDir()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, builtinSubfolder), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, builtinSubfolder, "_data.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "cloudfilesIndex.json"), []byte(`{"_data.txt":{"id":1,"key":"game/abc"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := BuildCloudfilesIndex(config, base)
	baseTime := time.Now()
	tick := 0
	store.SetReplayOpts(CloudFilesReplayOpts{
		Enabled:      true,
		MaxFileSize:  100,
		MaxTotalSize: 5,
		Expiry:       -1,
		Now: func() time.Time {
			tick++
			return baseTime.Add(time.Duration(tick) * time.Minute)
		},
	})
	t.Cleanup(store.Close)

	if _, err := writeReplay(store, "replays/1/0/a.gz", "aaa"); err != nil {
		t.Fatal(err)
	}
	if _, err := writeReplay(store, "replays/2/0/b.gz", "bbb"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get("replays/1/0/a.gz"); ok {
		t.Fatal("the oldest replay was not evicted")
	}
	if data, err := store.ReadFile("builtin/_data.txt"); err != nil || string(data) != "asset" {
		t.Fatalf("eviction damaged the builtin asset: %q, %v", data, err)
	}
}

func TestConcurrentPutsToSameKeyLeaveOneWholeFile(t *testing.T) {
	store, base := newTestCloudStore(t)
	key := "replays/77/0/same.gz"
	payloads := []string{strings.Repeat("A", 64*1024), strings.Repeat("B", 64*1024)}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, payload := range payloads {
		wg.Add(1)
		go func(content string) {
			defer wg.Done()
			<-start
			if _, err := writeReplay(store, key, content); err != nil {
				t.Errorf("Put: %v", err)
			}
		}(payload)
	}
	close(start)
	wg.Wait()

	content, err := os.ReadFile(filepath.Join(base, key))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != payloads[0] && string(content) != payloads[1] {
		t.Fatalf("final file is a mix of both uploads (%d bytes)", len(content))
	}
	parts, err := filepath.Glob(filepath.Join(base, "replays/77/0/*.part-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("orphan temporaries: %v", parts)
	}
}

func TestConcurrentPutAndReadsAreRaceFree(t *testing.T) {
	store, _ := newTestCloudStore(t)
	store.mu.Lock()
	store.Value["builtin/_data.txt"] = CloudfilesIndex{Key: "game/abc", Length: 1, Version: cloudfilesVersion}
	store.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for idx := 0; idx < 200; idx++ {
			_, _ = store.Get("builtin/_data.txt")
			for range store.Values() {
			}
			_, _, _ = store.GetByKey("game/abc")
		}
	}()
	for idx := 0; idx < 50; idx++ {
		key := "replays/" + strconv.Itoa(idx) + "/0/x.gz"
		if _, err := writeReplay(store, key, "payload"); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	<-done
}

func TestCloudFilesDisabledSafely(t *testing.T) {
	store := BuildCloudfilesIndex(t.TempDir(), "")
	if store.Enabled() {
		t.Fatal("an empty base folder must leave the store disabled")
	}
	if store.Credentials == nil {
		t.Fatal("credentials must be initialized even when disabled")
	}
	store.SetReplayOpts(CloudFilesReplayOpts{Enabled: true})
	if _, err := store.ReadFile("builtin/_data.txt"); err == nil {
		t.Fatal("ReadFile on a disabled store must fail")
	}
	if _, _, ok := store.Open("replays/1/0/x.gz"); ok {
		t.Fatal("Open on a disabled store must fail")
	}
	if _, err := writeReplay(store, "replays/1/0/x.gz", "x"); !errors.Is(err, ErrReplaysDisabled) {
		t.Fatalf("Put = %v, want ErrReplaysDisabled", err)
	}
	if _, _, ok := store.Credential("replays/1/0/x.gz"); ok {
		t.Fatal("Credential must fail on a disabled store")
	}
	store.Close()
	store.Close() // idempotent
}

func TestReplaysDisabledRejectsWritesButKeepsAssets(t *testing.T) {
	config := t.TempDir()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, builtinSubfolder), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, builtinSubfolder, "_data.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "cloudfilesIndex.json"), []byte(`{"_data.txt":{"id":1,"key":"game/abc"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := BuildCloudfilesIndex(config, base)
	store.SetReplayOpts(CloudFilesReplayOpts{Enabled: false})
	t.Cleanup(store.Close)

	if _, err := writeReplay(store, "replays/1/0/x.gz", "x"); !errors.Is(err, ErrReplaysDisabled) {
		t.Fatalf("Put = %v, want ErrReplaysDisabled", err)
	}
	if data, err := store.ReadFile("builtin/_data.txt"); err != nil || string(data) != "asset" {
		t.Fatalf("builtin asset became unreadable: %q, %v", data, err)
	}
}
