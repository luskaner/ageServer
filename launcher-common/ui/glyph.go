package ui

import (
	"github.com/charmbracelet/x/ansi"
)

// Name identifies one entry of the glyph catalogue.
type Name string

// The catalogue. Every marker used anywhere in the launcher modules is named
// here; the character itself never appears in the calling code, so a message
// written once reads the same in a modern terminal and in a Windows 7 cmd.
const (
	GlyphSuccess  Name = "Success"
	GlyphFailure  Name = "Failure"
	GlyphWarn     Name = "Warn"
	GlyphInfo     Name = "Info"
	GlyphProcess  Name = "Process"
	GlyphQuestion Name = "Question"
	GlyphBullet   Name = "Bullet"
	GlyphArrow    Name = "Arrow"
	GlyphBranch   Name = "Branch"
	GlyphLast     Name = "Last"

	GlyphCornerTL Name = "CornerTL"
	GlyphCornerTR Name = "CornerTR"
	GlyphCornerBL Name = "CornerBL"
	GlyphCornerBR Name = "CornerBR"
	GlyphRocket   Name = "Rocket"
)

// Glyph is one entry of the catalogue, with the three renderings it can take.
type Glyph struct {
	// ASCII is a purely ASCII token with no control characters.
	//
	// Every outcome marker is exactly six cells wide, so the text that follows it
	// starts at the same column whatever the outcome is. A catalogue where "[OK]"
	// is four and "[FAIL]" is six turns a column of outcomes into a staircase.
	ASCII string
	// Unicode is a single cell character, from the code page or from the font.
	// Single cell on purpose: one glyph per marker instead of three, and the line
	// stays short enough to read.
	Unicode string
	// Emoji is the rendering for a terminal with an emoji font.
	//
	// It is the single cell glyph in almost every case, and only the information
	// marker is an emoji. A double width glyph takes two of every line's cells and
	// shifts the whole message right, and a line of large colour blocks reads as
	// decoration rather than as an outcome; neither is worth a wider layout for one
	// marker that a single cell symbol already says.
	Emoji string
}

// asciiMarkerWidth is the width of every ASCII outcome marker. The tokens are
// padded to it on purpose, so a column of lines with different outcomes has its
// text starting in the same place.
const asciiMarkerWidth = 6

// glyphs is the closed catalogue. Order does not matter, it is a map.
//
// The three columns are the three levels of graceful degradation, and they are
// chosen so that each one is readable in the place it is most likely to end up:
//
//   - ASCII for a Windows 7 cmd.exe, a serial console, a pipe and a log file. The
//     tokens are all six cells, which is what keeps the text column straight, and
//     they are the form a screen reader reads out as a word.
//   - Unicode for a console with a UTF-8 code page, single cell throughout.
//   - Emoji for a terminal with a font that has them, which differs only in the
//     information marker.
//
// Not one marker may carry the Unicode Emoji property, and that is the whole reason
// the check and the cross are U+2713 and U+2717 rather than the U+2714 and U+2716
// that look identical. An Emoji character is routed to the colour emoji font:
// Windows Terminal hands ✔ and ✖ to Segoe UI Emoji, which paints them with its own
// palette, and the SGR colour in the escape sequence is parsed, discarded, and
// replaced by whatever magenta that font uses. U+2713 and U+2717 have no Emoji
// property, so no shaper ever asks for a colour glyph and the ANSI colour is what
// the reader sees. Verified against emoji-data.txt, where 2714 and 2716 are listed
// as Emoji and 2713 and 2717 are not.
//
// In progress is a spinner rather than a glyph wherever the console can animate
// one, so the static entry here is only what a line that cannot redraw falls back
// to. See Spinner.
var glyphs = map[Name]Glyph{
	GlyphSuccess:  {ASCII: "[ OK ]", Unicode: "✓", Emoji: "✓"},
	GlyphFailure:  {ASCII: "[FAIL]", Unicode: "✗", Emoji: "✗"},
	GlyphWarn:     {ASCII: "[WARN]", Unicode: "▲", Emoji: "▲"},
	GlyphInfo:     {ASCII: "[INFO]", Unicode: "•", Emoji: "🔹"},
	GlyphProcess:  {ASCII: "[WAIT]", Unicode: "»", Emoji: "»"},
	GlyphQuestion: {ASCII: "[?   ]", Unicode: "?", Emoji: "?"},
	GlyphBullet:   {ASCII: "-", Unicode: "•", Emoji: "•"},
	GlyphArrow:    {ASCII: "\t", Unicode: "↳", Emoji: "↳"},
	GlyphBranch:   {ASCII: "+-", Unicode: "├─", Emoji: "├─"},
	GlyphLast:     {ASCII: "-", Unicode: "└─", Emoji: "└─"},

	GlyphCornerTL: {ASCII: "+", Unicode: "╭", Emoji: "╭"},
	GlyphCornerTR: {ASCII: "+", Unicode: "╮", Emoji: "╮"},
	GlyphCornerBL: {ASCII: "+", Unicode: "╰", Emoji: "╰"},
	GlyphCornerBR: {ASCII: "+", Unicode: "╯", Emoji: "╯"},
	GlyphRocket:   {ASCII: "*", Emoji: "*"},
}

// glyphOrder is the catalogue order, used by the tests so failures are stable.
var glyphOrder = []Name{
	GlyphSuccess, GlyphFailure, GlyphWarn, GlyphInfo, GlyphProcess, GlyphQuestion,
	GlyphBullet, GlyphArrow, GlyphBranch, GlyphLast,
	GlyphCornerTL, GlyphCornerTR, GlyphCornerBL, GlyphCornerBR, GlyphRocket,
}

// G returns the effective glyph for the current tier, degrading through emoji,
// Unicode and ASCII until something exists.
func G(name Name) string {
	g := glyphs[name]
	switch Current().Tier {
	case TierEmoji:
		if g.Emoji != "" {
			return g.Emoji
		}
		fallthrough
	case TierUnicode:
		if g.Unicode != "" {
			return g.Unicode
		}
	}
	return g.ASCII
}

// markerFor renders a glyph as a line prefix: the glyph and the single space
// that follows it. The colour is applied by the caller, so that only the glyph
// carries it and the message text does not.
func markerFor(name Name) (prefix string, indent int) {
	g := G(name)
	return g + " ", ansi.StringWidth(g) + 1
}

// glyphWidth is the display width of the current rendering of a glyph.
func glyphWidth(name Name) int {
	return ansi.StringWidth(G(name))
}

// GlyphName exposes the catalogue for tests without letting callers mutate it.
func GlyphNames() []Name {
	names := make([]Name, len(glyphOrder))
	copy(names, glyphOrder)
	return names
}

// Lookup returns the catalogue entry for a name.
func Lookup(name Name) Glyph { return glyphs[name] }

// asciiOnly reports whether s contains nothing outside the byte range TierASCII
// allows, plus the two whitespace controls \t and \n.
func asciiOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\t' || b == '\n' {
			continue
		}
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}

// isASCIIGlyph reports whether the ASCII field is safe to emit on a console that
// can only show 0x20-0x7E, plus the tab.
//
// The tab is the one non printable byte the catalogue is allowed, because it is
// how the sub-step marker degrades: a terminal cannot draw "↳" out of ASCII, but it
// can indent, and an indented line says "this belongs to the one above" as well as
// an arrow does.
func (g Glyph) isASCIIGlyph() bool {
	if g.ASCII == "" {
		return false
	}
	return asciiOnly(g.ASCII)
}
