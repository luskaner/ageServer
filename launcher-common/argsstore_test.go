package launcher_common

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTempStore(t *testing.T) (*ArgsStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "args.txt")
	return NewArgsStore(path), path
}

func TestArgsStoreLoadMissingIsClean(t *testing.T) {
	store, _ := newTempStore(t)
	err, flags := store.Load()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("flags = %v, want empty", flags)
	}
}

// Regression: a zero-byte file (created but crashed before writing) produced a
// phantom [""] flag, which downstream made ConfigRevert attempt a full
// all-games revert instead of a no-op.
func TestArgsStoreLoadEmptyFileYieldsNoFlags(t *testing.T) {
	store, path := newTempStore(t)
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	err, flags := store.Load()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("empty file produced phantom flags %v", flags)
	}
}

func TestArgsStoreStoreLoadRoundTripAndDedup(t *testing.T) {
	store, _ := newTempStore(t)

	if err := store.Store([]string{"--a", "--b"}); err != nil {
		t.Fatal(err)
	}
	// Storing again with overlap must not duplicate.
	if err := store.Store([]string{"--b", "--c"}); err != nil {
		t.Fatal(err)
	}

	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"--a": true, "--b": true, "--c": true}
	if len(flags) != len(want) {
		t.Fatalf("flags = %v, want exactly %v", flags, want)
	}
	for _, f := range flags {
		if !want[f] {
			t.Fatalf("unexpected flag %q in %v", f, flags)
		}
	}
}

func TestArgsStoreDeleteIdempotent(t *testing.T) {
	store, path := newTempStore(t)
	if err := store.Store([]string{"--x"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("file not removed")
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("second Delete must be nil, got %v", err)
	}
}

func TestArgsStoreConcurrentSameInstance(t *testing.T) {
	store, _ := newTempStore(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			flag := "--flag" + string(rune('a'+i))
			_ = store.Store([]string{flag})
			_, _ = store.Load()
		}(i)
	}
	wg.Wait()
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) < 1 {
		t.Fatal("expected at least one stored flag")
	}
}

func TestArgsStoreTruncateOnShorterRewrite(t *testing.T) {
	store, path := newTempStore(t)
	// Simulate corrupted file with leading/trailing separators (5 bytes)
	if err := os.WriteFile(path, []byte("|--a|"), 0644); err != nil {
		t.Fatal(err)
	}
	// Storing the same flag should rewrite file to exactly "--a" (3 bytes),
	// not leave residual bytes like "--aa|".
	if err := store.Store([]string{"--a"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "--a" {
		t.Fatalf("file not truncated, got %q want %q", string(raw), "--a")
	}
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0] != "--a" {
		t.Fatalf("flags after truncate = %v, want [--a]", flags)
	}
}

func TestArgsStoreDedupWithinSameCall(t *testing.T) {
	store, _ := newTempStore(t)
	if err := store.Store([]string{"--x", "--x", "--y", "--x"}); err != nil {
		t.Fatal(err)
	}
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 unique flags, got %v", flags)
	}
	want := map[string]bool{"--x": true, "--y": true}
	for _, f := range flags {
		if !want[f] {
			t.Fatalf("unexpected %q in %v", f, flags)
		}
	}
	// Second store with duplicates should not create additional entries
	if err := store.Store([]string{"--x", "--y", "--y"}); err != nil {
		t.Fatal(err)
	}
	err, flags = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 2 {
		t.Fatalf("duplicate store grew unexpectedly: %v", flags)
	}
}

func TestArgsStoreEmptyFlagIgnored(t *testing.T) {
	store, path := newTempStore(t)
	if err := store.Store([]string{"", "--a", ""}); err != nil {
		t.Fatal(err)
	}
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0] != "--a" {
		t.Fatalf("empty flags should be ignored, got %v", flags)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "--a" {
		t.Fatalf("raw file should not contain separators for empty flags, got %q", string(raw))
	}
}

func TestArgsStoreByteToStringSliceEdgeCases(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"|", 0},
		{"||", 0},
		{"|a|", 1},
		{"a||b|||c", 3},
		{"--a|--b", 2},
		{"--a|", 1},
		{"|--a", 1},
	}
	for _, tc := range cases {
		got := argsStoreByteToStringSlice([]byte(tc.input))
		if len(got) != tc.want {
			t.Errorf("argsStoreByteToStringSlice(%q) = %v (len %d), want len %d", tc.input, got, len(got), tc.want)
		}
	}
}

func TestArgsStoreLoadWithSeparators(t *testing.T) {
	store, path := newTempStore(t)
	if err := os.WriteFile(path, []byte("--a||--b|"), 0644); err != nil {
		t.Fatal(err)
	}
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 flags, got %v", flags)
	}
}

func TestArgsStoreConcurrentStressNoDuplicates(t *testing.T) {
	store, _ := newTempStore(t)
	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = store.Store([]string{"--flag"})
			// each goroutine stores the same flag; dedup should keep 1
		}(i)
	}
	wg.Wait()
	err, flags := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0] != "--flag" {
		t.Fatalf("concurrent same flag dedup failed, got %v", flags)
	}
}
