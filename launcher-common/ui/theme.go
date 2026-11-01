package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Styles resolved once for the current Capability. With Profile at or below ASCII
// every one of them is the zero style, so Render is the identity and the
// degradation is structural rather than heuristic.
//
// The palette is the "base text neutral, coloured indicators" one: the body of a
// message stays in whatever the terminal's own foreground is, and only the marker
// in front of it carries a colour. A whole line of green saturates and reads as an
// alarm; a green tick followed by ordinary text does not.
var (
	stTitle   lipgloss.Style
	stVersion lipgloss.Style
	stOK      lipgloss.Style
	stFail    lipgloss.Style
	stWarn    lipgloss.Style
	stInfo    lipgloss.Style
	stProcess lipgloss.Style
	stDim     lipgloss.Style
	stKey     lipgloss.Style
	stValue   lipgloss.Style
	stBanner  lipgloss.Style
	stMark    lipgloss.Style
	stChip    lipgloss.Style
	stPath    lipgloss.Style
	stFlag    lipgloss.Style
)

// base is the style every other one derives from.
//
// TabWidth(lipgloss.NoTabConversion) is mandatory: lipgloss replaces every tab
// with four spaces when rendering, which would rewrite the indentation the
// existing messages rely on.
func base() lipgloss.Style {
	return lipgloss.NewStyle().TabWidth(lipgloss.NoTabConversion)
}

func initTheme(c Capability) {
	if c.Profile <= colorprofile.ASCII {
		stTitle, stVersion, stOK, stFail, stWarn = base(), base(), base(), base(), base()
		stInfo, stProcess, stDim, stKey, stValue = base(), base(), base(), base(), base()
		stBanner, stMark, stChip, stPath, stFlag = base(), base(), base(), base(), base()
		return
	}

	// Every profile that can do 16 colours is given the 16 colour palette, and
	// only a truecolor terminal is given the exact hex. That is deliberate: the
	// 256 colour range is a superset, but a terminal that has it also has the
	// basic sixteen, so asking for "45" buys a shade nobody asked for and costs
	// the guarantee that the same message reads the same on every terminal with
	// 16 colours. The exact hex is spent where it is actually available.
	//
	// No purple anywhere: on a dark background it is the lowest contrast colour
	// available and it reads as a rendering bug rather than as emphasis.
	ansiOnly := c.Profile < colorprofile.TrueColor
	// The colour a profile gets, chosen without asking Complete anything: an
	// index for the sixteen basic colours, or the exact hex for truecolor.
	colour := func(ansiIndex string, hex string) color.Color {
		if ansiOnly {
			return lipgloss.Color(ansiIndex)
		}
		return lipgloss.Color(hex)
	}
	accent := colour("14", "#8BE9FD")
	ok := colour("10", "#50FA7B")
	warn := colour("11", "#FFB86C")
	fail := colour("9", "#FF5555")
	mute := colour("8", "#6272A4")
	value := colour("7", "#888888")
	// The chip behind a component name is dark on purpose: it has to stay legible
	// over whatever the terminal's own foreground happens to be.
	chip := colour("4", "#2F4257")
	// Bright, for the header only. On a sixteen colour terminal this is ANSI 15,
	// which is the one colour that cannot be confused with the accent the headings
	// use.
	bright := colour("15", "#FFFFFF")

	stTitle = base().Foreground(accent).Bold(true)
	stVersion = base().Foreground(mute)
	stOK = base().Foreground(ok)
	stFail = base().Foreground(fail)
	stWarn = base().Foreground(warn)
	stInfo = base().Foreground(accent)
	// In progress is not an outcome, so it does not get an outcome colour, and it
	// does not get the heading colour either: a section title and a step marker in
	// the same cyan say the same thing twice, and the reader has to work out which
	// is which. Secondary says "this is happening" without pretending to be a
	// result.
	stProcess = base().Foreground(mute)
	// Secondary information: present, never competing with the message above it.
	stDim = base().Foreground(mute)
	stKey = base().Foreground(mute)
	stValue = base().Foreground(value)
	// The header is not a heading. It shared the accent with stTitle, which made
	// the program name look like the first category of the run; the brightest
	// colour in the palette belongs to the thing the whole output is about, and the
	// category names keep the accent to themselves.
	stBanner = base().Foreground(bright).Bold(true)
	// And the glyph in front of it is decoration, not content, so it is the
	// quietest thing on the line.
	stMark = base().Foreground(mute)
	stChip = base().Background(chip).Bold(true)
	stPath = base().Foreground(value).Italic(true)
	// Not Underline: lipgloss renders an underlined run with one escape pair per
	// character, which turns a flag into a dozen sequences. The dashes already
	// say "this is a flag", so bold is all the emphasis it needs.
	stFlag = base().Foreground(value).Bold(true)
}

// styleFor returns the colour a marker of the given kind uses. The message body
// itself is left in the terminal's own foreground.
func styleFor(name Name) lipgloss.Style {
	switch name {
	case GlyphSuccess:
		return stOK
	case GlyphFailure:
		return stFail
	case GlyphWarn:
		return stWarn
	case GlyphProcess:
		return stProcess
	default:
		return stInfo
	}
}
