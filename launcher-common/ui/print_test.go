package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// blockFuncs is every marked renderer, so the table below covers all of them.
func blockFuncs() map[string]func(string, ...any) string {
	return map[string]func(string, ...any) string{
		"Ok":     Ok,
		"Fail":   Fail,
		"Warn":   Warn,
		"Info":   Info,
		"Step":   Step,
		"Detail": Detail,
	}
}

// The TierASCII contract on every renderer: pure ASCII, the marker once and at
// the start of the line, and the input text intact.
func TestPrintTierASCII(t *testing.T) {
	markers := map[string]string{
		"Ok": "[ OK ]", "Fail": "[FAIL]", "Warn": "[WARN]",
		"Info": "[INFO]", "Step": "[WAIT]",
	}
	for name, fn := range blockFuncs() {
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSuffix(render(t, at(colorprofile.NoTTY, TierASCII, 80), func() {
				Println(fn("Successfully added user certificate"))
			}), "\n")
			if !AllASCII(got) {
				t.Errorf("output is not pure ASCII: %q", got)
			}
			if ContainsANSI(got) {
				t.Errorf("output carries an escape sequence: %q", got)
			}
			if !strings.Contains(got, "Successfully added user certificate") {
				t.Errorf("the message was altered: %q", got)
			}
			if marker, ok := markers[name]; ok {
				if strings.Count(got, marker) != 1 {
					t.Errorf("marker %q must appear exactly once, got %q", marker, got)
				}
				if !strings.Contains(got, baseIndent+marker+" ") {
					t.Errorf("the marker must lead the line, got %q", got)
				}
			}
		})
	}
}

// The same message must read well in all three tiers, and never lose a word.
func TestPrintEveryTierKeepsTheMessage(t *testing.T) {
	const message = "Failed to back up metadata"
	for _, tier := range []GlyphTier{TierASCII, TierUnicode, TierEmoji} {
		for _, columns := range []int{0, 40, 80} {
			got := render(t, at(colorprofile.NoTTY, tier, columns), func() {
				Println(Fail("%s", message))
			})
			if !strings.Contains(StripANSI(got), message) {
				t.Errorf("tier %d columns %d lost the message: %q", tier, columns, got)
			}
			if tier == TierASCII && !AllASCII(got) {
				t.Errorf("tier ASCII columns %d emitted a non ASCII byte: %q", columns, got)
			}
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				if columns > 0 && lineWidth(line) > columns {
					t.Errorf("tier %d columns %d produced a %d cell line: %q", tier, columns, lineWidth(line), line)
				}
			}
		}
	}
}

// With an unknown width nothing is wrapped, because an artificial line break in
// a file or a pipe only corrupts it.
func TestNoWrappingWithoutAColumnCount(t *testing.T) {
	const message = "Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when needed."
	got := StripANSI(render(t, at(colorprofile.NoTTY, TierASCII, 0), func() {
		Println(Warn("%s", message))
	}))
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("want a single line, got:\n%s", got)
	}
	wide := render(t, at(colorprofile.NoTTY, TierASCII, 60), func() {
		Println(Warn("%s", message))
	})
	if !strings.Contains(wide, "\n") {
		t.Fatalf("a 60 column console should have wrapped:\n%s", wide)
	}
}

// The continuation of a marked message has to line up under its text, so a long
// message still reads as one block.
func TestContinuationIsIndentedUnderTheText(t *testing.T) {
	const message = "Running as administrator, this is not recommended for security reasons. It will request isolated admin privileges if/when needed."
	got := render(t, at(colorprofile.NoTTY, TierASCII, 60), func() {
		Println(Warn("%s", message))
	})
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("want several lines, got:\n%s", got)
	}
	want := strings.Repeat(" ", len(baseIndent)+len(":-/ "))
	if !strings.HasPrefix(lines[1], want) {
		t.Errorf("continuation must start at column %d, got %q", len(want), lines[1])
	}
}

// Detail belongs to the message above it, so it sits under the marker text.
func TestDetailIsSubordinated(t *testing.T) {
	got := render(t, at(colorprofile.NoTTY, TierASCII, 80), func() {
		Println(Fail("Failed to back up metadata"))
		Println(Detail("Error message: %s", "disk on fire"))
	})
	want := []string{
		baseIndent + "[FAIL] Failed to back up metadata",
		strings.Repeat(" ", detailIndent()) + "Error message: disk on fire",
	}
	if got != strings.Join(want, "\n")+"\n" {
		t.Fatalf("got:\n%q\nwant:\n%q", got, strings.Join(want, "\n")+"\n")
	}
}

// Nothing is tabulated: a key and its value are one sentence, and the value starts
// right after the separator whatever the key is called. A column of whitespace on
// every line is a column the reader has to cross to reach the value.
func TestKVIsNotTabulated(t *testing.T) {
	for _, tier := range []GlyphTier{TierASCII, TierUnicode, TierEmoji} {
		t.Run(glyphTierName(tier), func(t *testing.T) {
			got := StripANSI(render(t, at(colorprofile.NoTTY, tier, 80), func() {
				KV(1, "main config file", "config.toml")
				KV(1, "game", "age2")
			}))
			lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("want two lines, got:\n%s", got)
			}
			want := []string{
				"  main config file: config.toml",
				"  game: age2",
			}
			for i, w := range want {
				if lines[i] != w {
					t.Errorf("line %d = %q, want %q", i, lines[i], w)
				}
			}
		})
	}
}

// A wrapped value lines up under the value, not under the key and not at the left
// margin, so the value reads as one block.
func TestKVWrapsUnderTheValue(t *testing.T) {
	got := StripANSI(render(t, at(colorprofile.NoTTY, TierASCII, 40), func() {
		KV(1, "game config file", strings.Repeat("long ", 12))
	}))
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the value should have wrapped:\n%s", got)
	}
	first := strings.Index(lines[0], "long")
	rest := strings.Index(lines[1], "long")
	if first < 0 || rest != first {
		t.Errorf("the continuation is at %d, want %d:\n%s", rest, first, got)
	}
}

func TestPrintfAndPrintlnAreTransparent(t *testing.T) {
	got := render(t, at(colorprofile.NoTTY, TierASCII, 40), func() {
		Printf("Dialog backend: %s.\n", "console")
		Println("Error:", "boom")
	})
	want := "Dialog backend: console.\nError: boom\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A percent sign in a message with no arguments must not be eaten.
func TestFormatWithoutArgumentsIsVerbatim(t *testing.T) {
	if got := format("100%% done"); got != "100%% done" {
		t.Errorf("got %q, want the text unchanged", got)
	}
	if got := format("100%% done"); got != "100%% done" {
		t.Errorf("got %q, want the text unchanged", got)
	}
	if got := format("%d%%", 50); got != "50%" {
		t.Errorf("got %q, want 50%%", got)
	}
}

// A phase heading is a blank line and a title. There is no rule: a line of
// dashes as wide as the window is a line the eye has to cross to reach the
// message under it.
func TestSectionIsAHeadingAndNothingElse(t *testing.T) {
	got := StripANSI(render(t, at(colorprofile.NoTTY, TierUnicode, 40), func() {
		Section("Phase")
	}))
	want := "\nPhase\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if ContainsANSI(got) {
		t.Errorf("a heading is not decorated per character: %q", got)
	}
}

func glyphTierName(t GlyphTier) string {
	switch t {
	case TierASCII:
		return "ascii"
	case TierUnicode:
		return "unicode"
	default:
		return "emoji"
	}
}
