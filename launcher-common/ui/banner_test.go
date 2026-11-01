package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// The banner is one line, so it only has to fit; it no longer has to be a box
// whose width matches its widest row.
func TestBannerFitsTheConsole(t *testing.T) {
	for _, columns := range []int{20, 40, 80, 200} {
		for _, tier := range []GlyphTier{TierASCII, TierUnicode, TierEmoji} {
			got := render(t, at(colorprofile.TrueColor, tier, columns), func() {
				Banner("AGE SERVER", "v1.4.0")
			})
			if strings.TrimSpace(got) == "" {
				t.Fatalf("columns %d tier %d printed nothing", columns, tier)
			}
			for _, l := range strings.Split(strings.TrimSpace(got), "\n") {
				if w := lineWidth(l); w > columns {
					t.Errorf("columns %d tier %d: a %d cell line does not fit: %q", columns, tier, w, l)
				}
			}
		}
	}
}

// TierASCII may only use characters a Windows 7 console has in code page 437.
func TestBannerIsPlainASCIIInTierASCII(t *testing.T) {
	got := render(t, at(colorprofile.NoTTY, TierASCII, 80), func() {
		Banner("AGE SERVER", "v1.4.0")
	})
	if !AllASCII(got) {
		t.Errorf("the ASCII banner left the ASCII range: %q", got)
	}
	if ContainsANSI(got) {
		t.Errorf("the ASCII banner emitted an escape sequence: %q", got)
	}
}

// A banner in a log file, or in a pipe, is noise: the width is unknown there.
func TestBannerIsNotPrintedWithoutAColumnCount(t *testing.T) {
	if got := render(t, at(colorprofile.NoTTY, TierASCII, 0), func() {
		Banner("AGE SERVER", "v1.4.0")
	}); got != "" {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestBannerWithoutVersion(t *testing.T) {
	got := render(t, at(colorprofile.NoTTY, TierASCII, 80), func() {
		Banner("AGE SERVER", "")
	})
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	// A leading blank line and the headline.
	if len(lines) != 2 || !strings.Contains(lines[1], "AGE SERVER") {
		t.Fatalf("want a blank line and one headline line, got %d:\n%s", len(lines), got)
	}
	if strings.Contains(strings.TrimSuffix(got, "\n"), "v1.4.0") {
		t.Errorf("an empty version must not leave a gap:\n%s", got)
	}
}

// The header used to be a box, and a box built around a name and a version is
// either padded with dead space or visibly broken, because the two are never the
// same width. One line has no such problem.
func TestBannerIsNeverABox(t *testing.T) {
	got := StripANSI(render(t, at(colorprofile.TrueColor, TierUnicode, 80), func() {
		Banner("launcher", "development")
	}))
	for _, box := range []string{"╭", "╮", "╰", "╯", "+", "|"} {
		if strings.Contains(got, box) {
			t.Errorf("the banner must not draw a box, found %q:\n%s", box, got)
		}
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Fatalf("want exactly one line, got %d:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], "development") {
		t.Errorf("the version is missing:\n%s", got)
	}
}

func TestBannerIsNotDoubleIndented(t *testing.T) {
	got := StripANSI(render(t, at(colorprofile.NoTTY, TierASCII, 80), func() {
		Banner("AGE SERVER", "v1.4.0")
	}))
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, baseIndent) {
			t.Errorf("every banner line starts at the left margin: %q", line)
		}
	}
}
