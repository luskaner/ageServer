package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// Regression: the numbered listing must stay byte for byte what it was, because
// select_server_test.go pins that exact text and because a Windows 7 cmd has
// neither the box drawing nor the column widths for a framed table.
func TestCandidatesStayPlainBelowTheUnicodeTier(t *testing.T) {
	for _, tier := range []ui.GlyphTier{ui.TierASCII} {
		restore := ui.SetCapability(ui.Capability{Profile: colorprofile.NoTTY, Tier: tier, Columns: 80})
		out := captureStdout(t, func() {
			printCandidates(testServers(3))
		})
		want := "Found the following servers:\n1. x\n2. xx\n3. xxx\n"
		if out != want {
			t.Fatalf("tier %d output =\n%q\nwant\n%q", tier, out, want)
		}
		restore()
	}
}

// A console that can draw box drawing gets the framed table instead, but it must
// still carry the same information: the index and the full description.
func TestCandidatesAreFramedAboveTheUnicodeTier(t *testing.T) {
	restore := ui.SetCapability(ui.Capability{Profile: colorprofile.TrueColor, Tier: ui.TierUnicode, Columns: 100})
	defer restore()
	out := captureStdout(t, func() {
		printCandidates(testServers(2))
	})
	for _, want := range []string{"1", "x", "2", "xx"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[info]") || strings.Contains(out, "[ ok ]") {
		t.Errorf("the table must not carry line markers, they break the columns:\n%s", out)
	}
	if !strings.Contains(out, "│") && !strings.Contains(out, "|") {
		t.Errorf("expected a framed table:\n%s", out)
	}
}

// The prompt gets the question marker in every tier; it is a line marker, not a
// column, so the double width emoji cannot misalign anything.
func TestPromptCarriesTheQuestionMarker(t *testing.T) {
	for _, tier := range []ui.GlyphTier{ui.TierASCII, ui.TierUnicode, ui.TierEmoji} {
		restore := ui.SetCapability(ui.Capability{Profile: colorprofile.NoTTY, Tier: tier, Columns: 80})
		out := captureStdout(t, func() {
			if _, ok := (consoleDialog{}).SelectServer(testServers(1), strings.NewReader("1\n")); !ok {
				t.Error("expected a selection")
			}
		})
		if !strings.Contains(out, "Enter the number of the server (1-1): ") {
			t.Errorf("tier %d lost the prompt:\n%s", tier, out)
		}
		if !strings.Contains(out, ui.G(ui.GlyphQuestion)) {
			t.Errorf("tier %d lost the question marker:\n%s", tier, out)
		}
		restore()
	}
}
