package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// minWrapWidth is the narrowest usable wrap target. Below it wrapping produces
// more noise than it removes, so the terminal is left to do it.
const minWrapWidth = 16

// breakPoints are the characters a wrapped line may break after. The path
// separators matter as much as the spaces: this output is full of Windows paths,
// and breaking "C:\Program" away from "Files\AgeServer" reads far worse than
// leaving the line a little long. The hyphen is there for the same reason: it is
// what joins the parts of a program name.
const breakPoints = " \t\\_/─-·,"

// wrapLines wraps text so that the widest line, once indent is prepended, fits in
// the console width. The returned lines carry no indent of their own.
//
// With an unknown width (Columns == 0, which is what a pipe or a file gets) the
// text is returned as is: inserting line breaks into redirected output only
// corrupts it.
func wrapLines(text string, indent int) []string {
	limit := Current().ColumnLimit()
	if limit <= 0 {
		return strings.Split(text, "\n")
	}
	width := limit - indent
	if width < minWrapWidth {
		return strings.Split(text, "\n")
	}
	return strings.Split(lipgloss.Wrap(text, width, breakPoints), "\n")
}

// indentLines prefixes every line with pad spaces.
func indentLines(lines []string, pad int) []string {
	if pad <= 0 {
		return lines
	}
	prefix := strings.Repeat(" ", pad)
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return lines
}

// trimTrailingLines removes the trailing spaces wrapping can leave behind.
func trimTrailingLines(lines []string) []string {
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

// join is the last step of a renderer that styles nothing per line.
func join(lines []string) string { return strings.Join(trimTrailingLines(lines), "\n") }

// lineWidth is the display width of a line, ignoring any escape sequence.
func lineWidth(s string) int { return lipgloss.Width(s) }

// truncate clips s to at most limit cells. A non positive limit yields an empty
// string, which is the only safe answer for a value that no longer fits.
func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if lineWidth(s) <= limit {
		return s
	}
	return ansi.Truncate(s, limit, "")
}
