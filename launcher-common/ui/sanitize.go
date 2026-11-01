package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// StripANSI removes every escape sequence from s. It is what guarantees, by
// construction, that a file log never receives a colour code, and what the
// parity tests compare on.
func StripANSI(s string) string {
	return ansi.Strip(s)
}

// ContainsANSI reports whether s carries an escape sequence.
func ContainsANSI(s string) bool {
	return strings.IndexByte(s, 0x1b) >= 0
}

// AllASCII reports whether s stays inside the printable ASCII range plus tab and
// newline, which is the TierASCII contract (P1).
func AllASCII(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	return asciiOnly(s)
}
