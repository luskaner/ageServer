package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// span renders one message line the way the console sees it, and reports the
// kinds it found in order.
func spansOf(t *testing.T, line string) ([]spanKind, string) {
	t.Helper()
	restore := with(t, at(colorprofile.TrueColor, TierEmoji, 0))
	defer restore()
	var kinds []spanKind
	for i := 0; i < len(line); {
		kind, n := spanAt(line, i)
		if n == 0 {
			i++
			continue
		}
		kinds = append(kinds, kind)
		i += n
	}
	return kinds, StripANSI(styleLine(lipgloss.NewStyle(), line))
}

func TestSpanClassification(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []spanKind
	}{
		{"Starting 'server'", []spanKind{spanComponent}},
		{"Starting server", []spanKind{spanComponent}},
		{"Server started.", []spanKind{spanComponent}},
		{"config-admin-agent is still active", []spanKind{spanComponent}},
		{"battle-server-manager could not be found", []spanKind{spanComponent}},
		{"the launcher-agent did not exit", []spanKind{spanComponent}},
		{"serverStart is false", []spanKind{spanKey}},
		{"servers were found", nil},
		{"Configuration is ready", nil},
		{"Server.Executable is not set", []spanKind{spanKey}},
		{"Use --output ascii instead", []spanKind{spanFlag}},
		{"Run the launcher", []spanKind{spanComponent}},
		{"found C:\\Program Files\\AgeServer", []spanKind{spanPath, spanPath}},
		{"read config.toml", []spanKind{spanPath}},
		{"mapping 192.168.1.50:27015 now", []spanKind{spanPath}},
		{"see https://example.com/x for more", []spanKind{spanPath}},
		{"and/or otherwise", nil},
		{"Ready.", nil},
	} {
		t.Run(tc.line, func(t *testing.T) {
			got, plain := spansOf(t, tc.line)
			// The invariant that matters most: styling must never change the text.
			if plain != tc.line {
				t.Errorf("StripANSI(rendered) = %q, want %q", plain, tc.line)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("kinds = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("kind %d = %d, want %d (all: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// Every line the catalogue can produce must survive the styler untouched: with
// the colour codes gone, what is left has to be exactly what the plain terminal
// would have shown.
func TestStylingNeverChangesTheText(t *testing.T) {
	catalogueMsgs := catalogue()
	plain := make([]string, len(catalogueMsgs))
	restore := with(t, at(colorprofile.NoTTY, TierEmoji, 0))
	for i, m := range catalogueMsgs {
		plain[i] = m.render(m.tmpl, m.args...)
	}
	restore()
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI} {
		restore := with(t, at(profile, TierEmoji, 0))
		for i, m := range catalogueMsgs {
			if got := StripANSI(m.render(m.tmpl, m.args...)); got != plain[i] {
				t.Errorf("profile %v: the colour changed the text\n got %q\nwant %q", profile, got, plain[i])
			}
		}
		restore()
	}
}

// The runs have to be styled one at a time. A nested reset would end the outer
// colour half way through the line, so every run has to carry its own terminator.
func TestRunsAreNotNested(t *testing.T) {
	restore := with(t, at(colorprofile.TrueColor, TierEmoji, 0))
	defer restore()
	line := styleLine(stOK, "  [ ok ] server started on C:\\srv")
	// Four runs: the lead in, the component, the middle and the path. Each one
	// carries its own terminator, which is what a nested render would break.
	if got := strings.Count(line, "\x1b[m"); got != 4 {
		t.Errorf("got %d terminators, want one per styled run:\n%q", got, line)
	}
	if got := StripANSI(line); got != "  [ ok ] server started on C:\\srv" {
		t.Errorf("the text changed: %q", got)
	}
}

// Below ANSI every style is the zero style, so a message must come out exactly as
// it went in. This is the property the whole parity contract rests on.
func TestNoStylingWithoutColour(t *testing.T) {
	for _, profile := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		restore := with(t, at(profile, TierEmoji, 0))
		for _, m := range catalogue() {
			if line := m.render(m.tmpl, m.args...); ContainsANSI(line) {
				t.Errorf("profile %v emitted an escape sequence: %q", profile, line)
			}
		}
		restore()
	}
}

func TestMatchIPv4(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"192.168.1.50 rest", 12},
		{"10.0.0.1", 8},
		{"999.1.1.1", 0},
		{"1.2.3", 0},
		{"1.2.3.4.5", 7}, // the trailing .5 is outside the address
		{"no address", 0},
	} {
		if got := matchIPv4(tc.in, 0); got != tc.want {
			t.Errorf("matchIPv4(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestTrimPunctuationLeavesTheAddressIntact(t *testing.T) {
	if got := trimPunctuation("192.168.1.50.", 0, 13); got != 12 {
		t.Errorf("got %d, want 12", got)
	}
	if got := trimPunctuation("C:\\srv\\", 0, 7); got != 6 {
		t.Errorf("got %d, want 6", got)
	}
}

func TestTokenBoundaries(t *testing.T) {
	// "a server here.": 0 "a", 1 space, 2..7 "server", 8 space, 9.. "here.".
	line := "a server here."
	for _, i := range []int{0, 2, 9} {
		if !beforeToken(line, i) {
			t.Errorf("index %d (%q) should start a token", i, line[i:i+1])
		}
	}
	for _, i := range []int{1, 3, 4, 8} {
		if beforeToken(line, i) {
			t.Errorf("index %d (%q) is inside a word", i, line[i:i+1])
		}
	}
	for _, i := range []int{1, 8, 14} {
		if !afterToken(line, i) {
			t.Errorf("index %d should end a token", i)
		}
	}
	for _, i := range []int{2, 3, 9, 10} {
		if afterToken(line, i) {
			t.Errorf("index %d is inside a word", i)
		}
	}
}
