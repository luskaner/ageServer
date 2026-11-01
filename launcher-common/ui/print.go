package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

// baseIndent is the left margin of every message, and it is zero.
//
// A left margin buys nothing here: the marker already separates the message from
// the left edge, a terminal starts at column zero anyway, and every space before a
// marker is a space the first word of the message does not get. The hierarchy is
// carried by the marker, by Detail for subordinate lines, and by Section for
// phases.
const baseIndent = ""

// detailIndent is the column a Detail line starts at, under the text that
// follows a marker. The ASCII marker is the widest one, so using it keeps all
// three tiers on the same column.
func detailIndent() int {
	return lipgloss.Width(baseIndent) + glyphWidth(GlyphProcess) + 1
}

// format applies fmt only when arguments were supplied, so a message with a
// stray percent sign and no arguments survives untouched.
func format(f string, a ...any) string {
	if len(a) == 0 {
		return f
	}
	return fmt.Sprintf(f, a...)
}

// block renders one marked message: the marker, then the text, with every
// continuation line aligned under the text.
//
// Only the marker is coloured. The body keeps the terminal's own foreground,
// because a whole line of green reads as an alarm and makes the next green line
// look like an error.
//
// The wording handed in is the exact wording used today. Nothing is rewritten,
// reordered or dropped, which is what makes the TierASCII output a superset of
// the current one instead of a rewrite of it.
func block(name Name, f string, a ...any) string {
	prefix, markerWidth := markerFor(name)
	indent := lipgloss.Width(baseIndent) + markerWidth
	lines := wrapLines(format(f, a...), indent)
	out := append([]string{baseIndent + styleFor(name).Render(prefix) + lines[0]}, indentLines(lines[1:], indent)...)
	return styleLines(base(), out)
}

// sub renders one unmarked message, indented under the message it belongs to.
// This is the level that says "this is a detail of the line above", which is what
// gives the flow its hierarchy.
func sub(text string) string {
	indent := detailIndent()
	return styleLines(stDim, indentLines(wrapLines(text, indent), indent))
}

// styleLines styles each laid out line on its own, so a long wrapped message keeps
// its styling from the first cell to the last.
func styleLines(base lipgloss.Style, lines []string) string {
	trimTrailingLines(lines)
	for i, line := range lines {
		lines[i] = styleLine(base, line)
	}
	return strings.Join(lines, "\n")
}

// Ok reports a completed operation.
func Ok(f string, a ...any) string { return block(GlyphSuccess, f, a...) }

// Fail reports a failed operation.
func Fail(f string, a ...any) string { return block(GlyphFailure, f, a...) }

// Warn reports a non blocking problem.
func Warn(f string, a ...any) string { return block(GlyphWarn, f, a...) }

// Info reports a note.
func Info(f string, a ...any) string { return block(GlyphInfo, f, a...) }

// Step reports an action that is being carried out. It has its own marker because
// "this is happening" and "this is a fact" are different things to read.
func Step(f string, a ...any) string { return block(GlyphProcess, f, a...) }

// Detail reports a line subordinate to the message above it, such as the error
// that explains the failure on the previous line.
func Detail(f string, a ...any) string { return sub(format(f, a...)) }

// Fault is a Detail that belongs to a failure: the error message, the exit code,
// the folder to look in.
//
// It is red because it is the part of a failure the reader acts on. A failure line
// in red followed by its reason in grey reads as two unrelated statements, and the
// reason is the one that says what went wrong.
func Fault(f string, a ...any) string {
	text := format(f, a...)
	indent := detailIndent()
	return styleLines(stFail, indentLines(wrapLines(text, indent), indent))
}

// Printf writes an already formatted message to the console. It neither wraps
// nor styles: the caller owns the text, which is what lets the launcher keep its
// file log byte for byte identical.
func Printf(f string, a ...any) { write(fmt.Sprintf(f, a...)) }

// Println writes a message to the console as is.
func Println(a ...any) { write(fmt.Sprintln(a...)) }

func write(s string) {
	w := Writer()
	if w == nil {
		return
	}
	_, _ = io.WriteString(w, s)
}

// line writes one rendered line followed by a newline.
func line(s string) { write(s + "\n") }

// Section is the heading of a phase. Phases are separated by one of these and by
// a blank line rather than by a rule: a line of dashes the width of the window is a
// line the eye has to cross to get to the message under it, which is the opposite
// of what a separator is for.
func Section(title string) {
	if title == "" {
		return
	}
	line("")
	line(stTitle.Render(baseIndent + title))
}

// kvSeparator joins a key and its value. A colon and a space, which is what the
// configuration files themselves use.
const kvSeparator = ": "

// KV prints a key and its value as one sentence. indent counts two-space steps to
// the right, 0 being the left margin every other message uses.
//
// There is no key column and nothing is aligned: the summary is three lines of
// text, not a table, and a table would need every key up front to line up and
// would put a column of whitespace between the reader and the value on every line.
func KV(indent int, key string, value string) {
	prefix := strings.Repeat("  ", max(0, indent))
	// The wrap column is the whole line, so a long path is split by the width of
	// the console rather than by a value column that is not there any more.
	lines := wrapLines(value, lipgloss.Width(prefix)+lipgloss.Width(key)+len(kvSeparator))
	// The key is always an identifier here, so it is styled as one whatever it
	// happens to be called; the value keeps whatever it mentions inside it.
	out := []string{stKey.Render(prefix+key+kvSeparator) + styleLine(base(), lines[0])}
	// Only the first line carries the key; a wrapped value lines up under the
	// value instead of restarting at the left margin.
	if len(lines) > 1 {
		out = append(out, styleLines(base(), indentLines(lines[1:], lipgloss.Width(prefix)+lipgloss.Width(key)+len(kvSeparator))))
	}
	line(join(out))
}

// Banner prints the program header on one line.
//
// It is one line and not a box because a box has to be exactly as wide as its
// widest row, and the widest row here is the version: a build called
// "development" is wider than the name, and any box drawn around both is either
// padded with dead space or visibly broken. version may be empty.
func Banner(program string, version string) {
	// Nothing at all when the width is unknown: a banner in a log file is noise.
	if Current().ColumnLimit() <= 0 {
		return
	}
	line("")
	available := Current().ColumnLimit() - lipgloss.Width(baseIndent)
	// The glyph is its own run so it can carry its own colour. Painted like the
	// name it competes with it, and painted like a section heading it makes the
	// header look like the first category of the run; as decoration it belongs to
	// neither.
	glyph := G(GlyphRocket)
	prefix := glyphWidth(GlyphRocket) + 1
	name := truncate(program, max(0, available-prefix))
	// The version is the least interesting thing on the line, so it gets the
	// quietest colour and whatever room is left over rather than pushing the name
	// out of view.
	version = truncate(version, max(0, available-prefix-lineWidth(name)-1))
	out := stMark.Render(glyph) + " " + stBanner.Render(name)
	if version != "" {
		// One space. The version is already a different colour and a different
		// weight, so a second space would be noise on top of the distinction.
		out += " " + stValue.Render(version)
	}
	line(baseIndent + out)
}
