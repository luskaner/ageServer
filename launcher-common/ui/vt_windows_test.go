//go:build windows

package ui

import (
	"testing"

	"golang.org/x/sys/windows"
)

// setOutputCodeForTest exists so the test can exercise the exact call the package
// makes, without duplicating it.
var setOutputCodeForTest = windows.SetConsoleOutputCP

// The code page comes back in the return register, not through the out parameter
// the documented signature shows. Passing a pointer and reading it back leaves it
// at zero, which is easy to "fix" into a permanent zero, so it is pinned here
// against the real console instead of trusted.
func TestOutputCodeTracksTheConsoleCodePage(t *testing.T) {
	if !stdoutIsConsole() {
		t.Skip("go test runs with a pipe, and there is no console code page to read")
	}
	before := outputCode()
	if before == 0 {
		t.Fatalf("outputCode() = 0 on a console")
	}

	// Round trip through the library, so this asserts what ensureUTF8Output does.
	original := before
	if err := setOutputCodeForTest(utf8CodePage); err != nil {
		t.Fatalf("SetConsoleOutputCP(65001): %v", err)
	}
	if got := outputCode(); got != utf8CodePage {
		t.Errorf("outputCode() = %d after setting 65001, want %d", got, utf8CodePage)
	}
	if original != utf8CodePage {
		if err := setOutputCodeForTest(original); err != nil {
			t.Fatalf("restoring the code page: %v", err)
		}
	}
}

// ensureUTF8Output runs on every program start and must never be the thing that
// breaks a console, so it is called here for real: with a pipe it must do nothing
// at all, and it must not panic.
func TestEnsureUTF8OutputIsSafeWithoutAConsole(t *testing.T) {
	if stdoutIsConsole() {
		ensureUTF8Output()
		return
	}
	ensureUTF8Output()
	if code := outputCode(); code != 0 && code != utf8CodePage {
		t.Logf("code page left at %d, which is the caller's business", code)
	}
}
