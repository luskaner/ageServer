//go:build windows

package main

import (
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestCStringToGo(t *testing.T) {
	if got := cStringToGo(0); got != "" {
		t.Fatalf("cStringToGo(0) = %q want empty", got)
	}
	ptr, err := windows.BytePtrFromString("hello world")
	if err != nil {
		t.Fatal(err)
	}
	// windows.BytePtrFromString allocates, need to keep alive
	got := cStringToGo(uintptr(unsafe.Pointer(ptr)))
	if got != "hello world" {
		t.Fatalf("cStringToGo = %q want hello world", got)
	}
}

func TestHeapFree_DoesNotPanic(t *testing.T) {
	// heapFree with 0 should not panic (HeapFree with NULL is allowed)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("heapFree(0) panicked: %v", r)
		}
	}()
	heapFree(0)
	// Also test with a valid heap allocation
	heap, _, _ := procGetProcessHeap.Call()
	if heap == 0 {
		t.Skip("GetProcessHeap failed")
	}
	// Allocate 16 bytes via HeapAlloc
	procHeapAlloc := kernel32.NewProc("HeapAlloc")
	ptr, _, _ := procHeapAlloc.Call(heap, 0, 16)
	if ptr == 0 {
		t.Skip("HeapAlloc failed")
	}
	heapFree(ptr)
}

func TestWindowsToUnixPath_Empty(t *testing.T) {
	_, err := WindowsToUnixPath("")
	if err == nil || err.Error() != "empty path" {
		t.Fatalf("expected empty path error, got %v", err)
	}
	_, err = WindowsToUnixPath("   ")
	if err == nil {
		t.Fatal("expected error for whitespace")
	}
}

func TestWindowsToUnixPath_NotWine(t *testing.T) {
	origFind := findWineProcFn
	defer func() { findWineProcFn = origFind }()
	findWineProcFn = func() error { return errors.New("not found") }

	_, err := WindowsToUnixPath(`C:\Users\test`)
	if err == nil || err.Error() != "wine_get_unix_file_name not available (not a Wine environment)" {
		t.Fatalf("expected not Wine error, got %v", err)
	}
}

func TestWindowsToUnixPath_UTF16Error(t *testing.T) {
	origFind := findWineProcFn
	origUTF16 := utf16PtrFromStringFn
	defer func() { findWineProcFn = origFind; utf16PtrFromStringFn = origUTF16 }()

	findWineProcFn = func() error { return nil }
	utf16PtrFromStringFn = func(s string) (*uint16, error) {
		return nil, errors.New("utf16 fail")
	}

	_, err := WindowsToUnixPath(`C:\test`)
	if err == nil || err.Error() != "UTF16PtrFromString: utf16 fail" {
		t.Fatalf("expected UTF16 error, got %v", err)
	}
}

func TestWindowsToUnixPath_SuccessNoTail(t *testing.T) {
	origFind := findWineProcFn
	origCall := callWineGetUnixFileNameFn
	origCString := cStringToGoFn
	origHeap := heapFreeFn
	defer func() {
		findWineProcFn = origFind
		callWineGetUnixFileNameFn = origCall
		cStringToGoFn = origCString
		heapFreeFn = origHeap
	}()

	findWineProcFn = func() error { return nil }
	// Mock Wine to return a valid Unix path for any input
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr {
		// Return a fake C string "/unix/path"
		b, _ := windows.BytePtrFromString("/unix/path")
		return uintptr(unsafe.Pointer(b))
	}
	cStringToGoFn = func(ptr uintptr) string {
		if ptr == 0 {
			return ""
		}
		return "/unix/path"
	}
	heapFreed := false
	heapFreeFn = func(ptr uintptr) {
		heapFreed = true
		if ptr == 0 {
			t.Error("heapFree called with 0")
		}
	}

	got, err := WindowsToUnixPath(`C:\Windows\test`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/unix/path" {
		t.Fatalf("got %q want /unix/path", got)
	}
	if !heapFreed {
		t.Error("heapFree not called")
	}
}

func TestWindowsToUnixPath_SuccessWithTail(t *testing.T) {
	origFind := findWineProcFn
	origCall := callWineGetUnixFileNameFn
	origCString := cStringToGoFn
	origHeap := heapFreeFn
	defer func() {
		findWineProcFn = origFind
		callWineGetUnixFileNameFn = origCall
		cStringToGoFn = origCString
		heapFreeFn = origHeap
	}()

	findWineProcFn = func() error { return nil }
	callCount := 0
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr {
		callCount++
		if callCount == 1 {
			// First call fails (return 0) for full path "C:\a\b\c"
			return 0
		}
		if callCount == 2 {
			// Second call for "C:\a\b" fails
			return 0
		}
		// Third call for "C:\a" succeeds
		b, _ := windows.BytePtrFromString("/unix/a")
		return uintptr(unsafe.Pointer(b))
	}
	cStringToGoFn = func(ptr uintptr) string { return "/unix/a" }
	heapFreeFn = func(uintptr) {}

	got, err := WindowsToUnixPath(`C:\a\b\c`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Should have trimmed "b" and "c" and joined
	expected := "/unix/a/b/c"
	if got != expected {
		t.Fatalf("got %q want %q", got, expected)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
}

func TestWindowsToUnixPath_InvalidChars(t *testing.T) {
	origFind := findWineProcFn
	origCall := callWineGetUnixFileNameFn
	defer func() { findWineProcFn = origFind; callWineGetUnixFileNameFn = origCall }()

	findWineProcFn = func() error { return nil }
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr { return 0 } // always fail

	_, err := WindowsToUnixPath(`C:\a\bad*file`)
	if err == nil || err.Error() != `invalid characters in path component: "bad*file"` {
		t.Fatalf("expected invalid chars error, got %v", err)
	}

	_, err = WindowsToUnixPath(`C:\a\bad?`)
	if err == nil {
		t.Fatal("expected invalid chars for ?")
	}
}

func TestWindowsToUnixPath_ReducedToEmpty(t *testing.T) {
	origFind := findWineProcFn
	origCall := callWineGetUnixFileNameFn
	defer func() { findWineProcFn = origFind; callWineGetUnixFileNameFn = origCall }()

	findWineProcFn = func() error { return nil }
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr { return 0 }

	_, err := WindowsToUnixPath(`C:\`)
	if err == nil {
		t.Fatal("expected reduced to empty error")
	}
	// After trimming C:\ -> C: -> then LastIndexAny fails, should be "wine could not resolve any part" or "reduced to empty"
	if err.Error() != "failed to convert path (wine could not resolve any part)" && err.Error() != "failed to convert path (reduced to empty)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWindowsToUnixPath_TrimsTrailingSeparators(t *testing.T) {
	origFind := findWineProcFn
	origCall := callWineGetUnixFileNameFn
	origCString := cStringToGoFn
	origHeap := heapFreeFn
	defer func() {
		findWineProcFn = origFind
		callWineGetUnixFileNameFn = origCall
		cStringToGoFn = origCString
		heapFreeFn = origHeap
	}()

	findWineProcFn = func() error { return nil }
	// For "C:\a\b\" -> first fails (empty tail), second fails for "C:\a\b" (adds "b"), third succeeds for "C:\a"
	callCount := 0
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr {
		callCount++
		if callCount <= 2 {
			return 0
		}
		b, _ := windows.BytePtrFromString("/unix/a")
		return uintptr(unsafe.Pointer(b))
	}
	cStringToGoFn = func(uintptr) string { return "/unix/a" }
	heapFreeFn = func(uintptr) {}

	got, err := WindowsToUnixPath(`C:\a\b\`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/unix/a/b" {
		t.Fatalf("got %q want /unix/a/b", got)
	}
}
