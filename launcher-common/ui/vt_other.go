//go:build !windows

package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// vtUsable is always true off Windows: every Unix terminal this can run on
// either understands ANSI or is already rejected by the TTY check.
func vtUsable() bool { return true }

// outputCode has no meaning off Windows.
func outputCode() uint32 { return 0 }

// ensureUTF8Output has nothing to do off Windows: the console code page is a
// Windows concept, and a Unix terminal is either UTF-8 already or handled by the
// locale.
func ensureUTF8Output() {}

// consoleColumns returns the terminal width, or 0 when it is unknown (a pipe or
// a file, which the isTerminal check has already ruled out anyway).
func consoleColumns() int {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return 0
	}
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 0
	}
	return w
}

// localeUTF8 reports whether the locale environment names a UTF-8 codeset.
func localeUTF8() bool {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := envValue(os.Environ(), name)
		if value == "" {
			continue
		}
		upper := strings.ToUpper(value)
		if strings.Contains(upper, "UTF-8") || strings.Contains(upper, "UTF8") {
			return true
		}
		// A non-empty locale that does not name UTF-8 decides the answer, so
		// LANG=C is not rescued by a UTF-8 LC_CTYPE further down.
		return false
	}
	return false
}

// osVersion has no meaning off Windows.
func osVersion() (major, build uint32) { return 0, 0 }
