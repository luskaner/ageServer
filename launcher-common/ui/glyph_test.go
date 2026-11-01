package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// TestGlyphASCIIIsPureASCII is the TierASCII guarantee applied to the catalogue:
// nothing can leak a byte the old code page cannot render.
func TestGlyphASCIIIsPureASCII(t *testing.T) {
	for _, name := range GlyphNames() {
		g := Lookup(name)
		if !g.isASCIIGlyph() {
			t.Errorf("%s ASCII = %q, want printable ASCII only (0x20-0x7E)", name, g.ASCII)
		}
	}
}

// TestGlyphUnicodeIsSingleWidth keeps the Unicode column usable as a line marker,
// where a fixed width is what makes the continuation indent line up. The tree
// connectors are the deliberate exception and are checked separately.
func TestGlyphUnicodeIsSingleWidth(t *testing.T) {
	twoCell := map[Name]bool{GlyphBranch: true, GlyphLast: true}
	for _, name := range GlyphNames() {
		u := Lookup(name).Unicode
		if u == "" {
			continue
		}
		want := 1
		if twoCell[name] {
			want = 2
		}
		if w := ansi.StringWidth(u); w != want {
			t.Errorf("%s Unicode = %q, width %d, want %d", name, u, w, want)
		}
		if e := Lookup(name).Emoji; e != "" && ansi.StringWidth(e) < 1 {
			t.Errorf("%s Emoji = %q, want a non zero width", name, e)
		}
	}
}

// The rich tier is single cell throughout, with one deliberate exception.
//
// A double width glyph eats two of every line's cells and shifts the message right
// under it, so the outcome markers stopped being emoji. The information marker is
// the one that was kept, and its width is pinned here because the continuation
// indent is computed from it: a marker that reports the wrong width drags every
// wrapped line out of alignment.
func TestGlyphEmojiIsSingleWidthExceptTheInformationMarker(t *testing.T) {
	// The tree connectors are two cells by nature, in every tier, and are not
	// line markers: they only ever sit inside a tree that is already aligned.
	twoCell := map[Name]bool{GlyphBranch: true, GlyphLast: true, GlyphInfo: true}
	for _, name := range GlyphNames() {
		e := Lookup(name).Emoji
		if e == "" {
			continue
		}
		width := ansi.StringWidth(e)
		want := 1
		if twoCell[name] {
			want = 2
		}
		if width != want {
			t.Errorf("%s Emoji = %q, width %d, want %d", name, e, width, want)
		}
	}
}

func TestGRespectsTier(t *testing.T) {
	for _, tier := range []GlyphTier{TierASCII, TierUnicode, TierEmoji} {
		with(t, at(colorprofile.NoTTY, tier, 80))
		for _, name := range GlyphNames() {
			g := Lookup(name)
			want := g.ASCII
			switch tier {
			case TierEmoji:
				if g.Emoji != "" {
					want = g.Emoji
				} else if g.Unicode != "" {
					want = g.Unicode
				}
			case TierUnicode:
				if g.Unicode != "" {
					want = g.Unicode
				}
			}
			if got := G(name); got != want {
				t.Errorf("G(%s) in tier %d = %q, want %q", name, tier, got, want)
			}
		}
	}
	// The ASCII token is the one a Windows 7 cmd has to show, so pin it down.
	with(t, at(colorprofile.NoTTY, TierASCII, 80))
	if got := G(GlyphSuccess); got != "[ OK ]" {
		t.Errorf("G(Success) in TierASCII = %q, want the ASCII token", got)
	}
}

// Every catalogue entry must be reachable from the order used by the tests,
// otherwise the table tests would silently skip it.
func TestGlyphNamesCoverTheCatalogue(t *testing.T) {
	if len(glyphOrder) != len(glyphs) {
		t.Fatalf("glyphOrder has %d entries, the catalogue has %d", len(glyphOrder), len(glyphs))
	}
	for _, name := range glyphOrder {
		if _, ok := glyphs[name]; !ok {
			t.Errorf("%s is listed but missing from the catalogue", name)
		}
	}
}

func TestAsciiOnly(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{":-)", true},
		{"a\tb\nc", true},
		{"", true},
		{":-) \x1b[32m", false},
		{":-) ✔", false},
		{":-)\x00", false},
		{"\u00e9", false},
	} {
		if got := asciiOnly(tc.in); got != tc.want {
			t.Errorf("asciiOnly(%q) = %t, want %t", tc.in, got, tc.want)
		}
	}
}

// The three levels of degradation, pinned so a change to one of them is a
// decision rather than an accident.
func TestOutcomeMarkerTable(t *testing.T) {
	for _, tc := range []struct {
		name                  Name
		ascii, unicode, emoji string
	}{
		{GlyphSuccess, "[ OK ]", "✓", "✓"},
		{GlyphFailure, "[FAIL]", "✗", "✗"},
		{GlyphWarn, "[WARN]", "▲", "▲"},
		{GlyphInfo, "[INFO]", "•", "🔹"},
		{GlyphProcess, "[WAIT]", "»", "»"},
	} {
		got := Lookup(tc.name)
		if got.ASCII != tc.ascii || got.Unicode != tc.unicode || got.Emoji != tc.emoji {
			t.Errorf("%s = %q/%q/%q, want %q/%q/%q", tc.name,
				got.ASCII, got.Unicode, got.Emoji, tc.ascii, tc.unicode, tc.emoji)
		}
	}
}

// Every outcome marker is six cells in ASCII and one in Unicode, so the text after
// it never moves sideways as the outcome changes.
func TestOutcomeMarkersHaveAUniformWidth(t *testing.T) {
	for _, name := range []Name{GlyphSuccess, GlyphFailure, GlyphWarn, GlyphInfo, GlyphProcess} {
		g := Lookup(name)
		if w := lipgloss.Width(g.ASCII); w != asciiMarkerWidth {
			t.Errorf("%s ASCII %q is %d cells, want %d", name, g.ASCII, w, asciiMarkerWidth)
		}
		if w := lipgloss.Width(g.Unicode); w != 1 {
			t.Errorf("%s Unicode %q is %d cells, want 1", name, g.Unicode, w)
		}
	}
}

// The sub-step marker cannot be drawn out of ASCII, so it degrades to what ASCII
// does have: indentation.
func TestSubStepMarkerDegradesToATab(t *testing.T) {
	if got := Lookup(GlyphArrow).ASCII; got != "\t" {
		t.Errorf("Arrow ASCII = %q, want a tab", got)
	}
	if got := Lookup(GlyphArrow).Unicode; got != "↳" {
		t.Errorf("Arrow Unicode = %q, want the arrow", got)
	}
	// A tab is the one control byte the TierASCII contract allows, and only here.
	if !Lookup(GlyphArrow).isASCIIGlyph() {
		t.Error("a tab is still inside the TierASCII contract")
	}
	for _, name := range GlyphNames() {
		if name == GlyphArrow {
			continue
		}
		if strings.ContainsRune(Lookup(name).ASCII, '\t') {
			t.Errorf("%s ASCII %q must not be a tab", name, Lookup(name).ASCII)
		}
	}
}

// A marker carrying the Unicode Emoji property is routed to the colour emoji font:
// Windows Terminal hands ✔ and ✖ to Segoe UI Emoji, which paints them with its own
// palette and throws the SGR colour away, so the marker comes out magenta whatever
// colour the message asked for. U+2714 and U+2716 are listed as Emoji in
// emoji-data.txt; U+2713 and U+2717 are not, which is why the catalogue uses those.
//
// The list is the set of code points Unicode currently marks as Emoji that a
// launcher marker could plausibly be, so a "harmless" swap is caught here instead of
// on somebody else's terminal.
func TestNoMarkerCarriesTheEmojiProperty(t *testing.T) {
	// From emoji-data.txt (2026-01-30): every code point Unicode marks as Emoji in
	// the blocks a marker could plausibly come from. U+2713 and U+2717, the ones
	// this catalogue uses, are absent from it; U+2714 and U+2716 are in it.
	const emojiProperty = "2714" + // ✔ heavy check mark
		"2716" + // ✖ heavy multiplication x
		"203C" + // ‼ double exclamation mark
		"2139" + // ℹ information
		"2049" + // ⁉ exclamation question mark
		"3030" + // 〰 wavy dash
		"303D" + // 〽 part alternation mark
		"3297" + // ㊗ congratulation button
		"3299" + // ㊙ secret button
		"2B50" // ⭐ star
	// U+1F539, the small blue diamond, is deliberately not in the list: it is
	// reserved for a proposal and has no Emoji property, so it is drawn as text.
	for _, name := range GlyphNames() {
		g := Lookup(name)
		for _, field := range []struct {
			tier, glyph string
		}{
			{"unicode", g.Unicode}, {"emoji", g.Emoji},
		} {
			if field.glyph == "" {
				continue
			}
			for _, r := range field.glyph {
				code := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(fmt.Sprintf("%U", r)), "U+"))
				if strings.Contains(emojiProperty, code) {
					t.Errorf("%s %s = %q: U+%s carries the Emoji property and will be drawn in the colour emoji font", name, field.tier, field.glyph, code)
				}
			}
		}
	}
}
