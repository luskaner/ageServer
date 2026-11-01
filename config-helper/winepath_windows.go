package main

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// Load kernel32.dll procedures
	kernel32                = windows.NewLazyDLL("kernel32.dll")
	procWineGetUnixFileName = kernel32.NewProc("wine_get_unix_file_name")
	procGetProcessHeap      = kernel32.NewProc("GetProcessHeap")
	procHeapFree            = kernel32.NewProc("HeapFree")
)

const invalidChars = `*?<>|"`

// heapFree releases memory using GetProcessHeap + HeapFree.
// It is assumed that GetProcessHeap and HeapFree exist in the environment.
func heapFree(ptr uintptr) {
	heap, _, _ := procGetProcessHeap.Call()
	_, _, _ = procHeapFree.Call(heap, 0, ptr)
}

// cStringToGo converts a C char* pointer returned by Wine to a Go string.
func cStringToGo(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	// Convert to *byte and use syscall.BytePtrToString for safe conversion.
	return windows.BytePtrToString((*byte)(unsafe.Pointer(ptr)))
}

var (
	findWineProcFn            = func() error { return procWineGetUnixFileName.Find() }
	callWineGetUnixFileNameFn = func(ptr uintptr) uintptr {
		ret, _, _ := procWineGetUnixFileName.Call(ptr)
		return ret
	}
	heapFreeFn      = heapFree
	cStringToGoFn   = cStringToGo
	utf16PtrFromStringFn = windows.UTF16PtrFromString
)

// WindowsToUnixPath attempts to convert a Windows path to its Unix equivalent under Wine.
// If the direct call fails, it trims path components from the right until Wine resolves a part,
// then reconstructs the full Unix path. All error messages are in English.
func WindowsToUnixPath(path string) (string, error) {
	// Validate the input before checking the environment so callers get
	// consistent validation errors regardless of where this runs.
	ntpath := strings.TrimSpace(path)
	if ntpath == "" {
		return "", errors.New("empty path")
	}
	// Ensure the wine_get_unix_file_name procedure exists (indicates a Wine environment).
	if err := findWineProcFn(); err != nil {
		return "", errors.New("wine_get_unix_file_name not available (not a Wine environment)")
	}
	var tail []string
	for {
		// Convert to UTF-16 pointer as Windows APIs expect UTF-16.
		ntpathPtr, err := utf16PtrFromStringFn(ntpath)
		if err != nil {
			return "", fmt.Errorf("UTF16PtrFromString: %w", err)
		}
		// Call wine_get_unix_file_name
		ret := callWineGetUnixFileNameFn(uintptr(unsafe.Pointer(ntpathPtr)))
		if ret != 0 {
			// Convert the returned C string to Go string first.
			unixName := cStringToGoFn(ret)
			// Free the memory returned by Wine immediately after conversion.
			heapFreeFn(ret)
			if len(tail) > 0 {
				// Join with '/' because this is a Unix path.
				return unixName + "/" + strings.Join(tail, "/"), nil
			}
			return unixName, nil
		}
		// If conversion failed, trim the last component and try again.
		lastSlash := strings.LastIndexAny(ntpath, `\/`)
		if lastSlash == -1 {
			return "", errors.New("failed to convert path (wine could not resolve any part)")
		}
		partCut := ntpath[lastSlash+1:]
		if partCut != "" {
			// Validate that the trimmed component does not contain invalid characters.
			if strings.IndexAny(partCut, invalidChars) != -1 {
				return "", fmt.Errorf("invalid characters in path component: %q", partCut)
			}
			// Prepend the trimmed component to tail to preserve original order.
			tail = append([]string{partCut}, tail...)
		}
		ntpath = ntpath[:lastSlash]
		ntpath = strings.TrimRight(ntpath, `\/`) // remove trailing separators
		if ntpath == "" {
			// If we've reduced to empty and still couldn't resolve, fail.
			return "", errors.New("failed to convert path (reduced to empty)")
		}
	}
}
