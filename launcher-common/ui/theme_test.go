package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// styles is every style initTheme defines, so a change to the set cannot slip
// past the tests below.
func styles() map[string]lipgloss.Style {
	return map[string]lipgloss.Style{
		"stTitle": stTitle, "stVersion": stVersion, "stOK": stOK, "stFail": stFail,
		"stWarn": stWarn, "stInfo": stInfo, "stDim": stDim, "stKey": stKey,
		"stValue": stValue, "stBanner": stBanner, "stChip": stChip, "stPath": stPath,
		"stFlag": stFlag,
	}
}

// In TierASCII every style must be the identity, so Render hands back exactly
// what it was given. That is what makes the degradation structural instead of
// heuristic: there is nothing to strip because nothing was ever added.
func TestThemeIsIdentityInASCII(t *testing.T) {
	with(t, at(colorprofile.NoTTY, TierASCII, 80))
	for name, style := range styles() {
		if got := style.Render("hola"); got != "hola" {
			t.Errorf("%s.Render = %q, want %q", name, got, "hola")
		}
		// NoTabConversion: lipgloss would otherwise turn this into four spaces,
		// silently rewriting the indentation the existing messages rely on.
		if got := style.Render("a\tb"); got != "a\tb" {
			t.Errorf("%s.Render(%q) = %q, want the tab left alone", name, "a\tb", got)
		}
	}
}

// NO_COLOR lands on colorprofile.ASCII, which still gets no escape sequence.
func TestThemeNoANSIWhenColorOff(t *testing.T) {
	for _, profile := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		with(t, at(profile, TierEmoji, 80))
		for name, style := range styles() {
			got := style.Render("hola")
			if ContainsANSI(got) {
				t.Errorf("profile %v: %s emitted an escape sequence: %q", profile, name, got)
			}
			if StripANSI(got) != "hola" {
				t.Errorf("profile %v: %s changed the text: %q", profile, name, got)
			}
		}
	}
}

func TestThemeColoursWhenOn(t *testing.T) {
	with(t, at(colorprofile.TrueColor, TierEmoji, 80))
	if !ContainsANSI(stOK.Render("hola")) {
		t.Error("with colour on, a style must emit a sequence")
	}
	// Complete downgrades an ideal colour to what the profile can express.
	with(t, at(colorprofile.ANSI, TierEmoji, 80))
	ok := stOK.Render("hola")
	if !ContainsANSI(ok) {
		t.Errorf("ANSI profile should still be able to render a colour, got %q", ok)
	}
	if strings.Contains(ok, "38;2;") {
		t.Errorf("the ANSI profile must not receive truecolor escapes: %q", ok)
	}
}

func TestStyleFor(t *testing.T) {
	withColor(t, TierEmoji)
	for _, tc := range []struct {
		name Name
		want lipgloss.Style
	}{
		{GlyphSuccess, stOK},
		{GlyphFailure, stFail},
		{GlyphWarn, stWarn},
		{GlyphInfo, stInfo},
		{GlyphProcess, stProcess},
		{GlyphQuestion, stInfo},
		{GlyphBullet, stInfo},
	} {
		if got := styleFor(tc.name); got.String() != tc.want.String() {
			t.Errorf("styleFor(%s) is not the expected style", tc.name)
		}
	}
}

// The outcome is the one thing a colour may carry, so it is pinned end to end:
// through the renderer, in every profile that can show colour, with the exact
// colour each outcome is supposed to be.
func TestMarkerColourFollowsTheOutcome(t *testing.T) {
	for _, profile := range []colorprofile.Profile{colorprofile.ANSI, colorprofile.ANSI256, colorprofile.TrueColor} {
		t.Run(profile.String(), func(t *testing.T) {
			restore := with(t, at(profile, TierUnicode, 80))
			defer restore()
			for _, tc := range []struct {
				name string
				got  string
				want string
			}{
				// The two the eye relies on most: a tick is green, a cross is red.
				{"ok", Ok("message"), stOK.Render(G(GlyphSuccess) + " ")},
				{"fail", Fail("message"), stFail.Render(G(GlyphFailure) + " ")},
				{"warn", Warn("message"), stWarn.Render(G(GlyphWarn) + " ")},
				{"info", Info("message"), stInfo.Render(G(GlyphInfo) + " ")},
				{"step", Step("message"), stProcess.Render(G(GlyphProcess) + " ")},
			} {
				if !strings.HasPrefix(tc.got, tc.want) {
					t.Errorf("%s marker is %q, want it to start with %q", tc.name, tc.got, tc.want)
				}
				// The body is not coloured: only the marker is, so a whole line of
				// green cannot make the next green line look like an error.
				if body := tc.got[len(tc.want):]; ContainsANSI(body) {
					t.Errorf("%s coloured the body: %q", tc.name, body)
				}
			}
		})
	}
}

// The palette, as the colour it actually resolves to. Green for a completed
// operation and red for a failure are the two that have to be right on a terminal
// that only has sixteen colours, which is every terminal that has colour at all.
func TestOutcomeColoursInSixteenColors(t *testing.T) {
	restore := with(t, at(colorprofile.ANSI, TierUnicode, 80))
	defer restore()
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"ok", stOK.Render("x"), "\x1b[92m"},
		{"fail", stFail.Render("x"), "\x1b[91m"},
		{"warn", stWarn.Render("x"), "\x1b[93m"},
		{"info", stInfo.Render("x"), "\x1b[96m"},
	} {
		if !strings.HasPrefix(tc.got, tc.want) {
			t.Errorf("%s = %q, want it to start with %q", tc.name, tc.got, tc.want)
		}
	}
}

// The header is not a heading, and a step is not a result. Both used to share the
// accent with the category titles, which made the program name read as the first
// category and the in-progress marker read as a heading.
func TestHeaderAndHeadingsDoNotShareTheAccent(t *testing.T) {
	restore := with(t, at(colorprofile.TrueColor, TierUnicode, 80))
	defer restore()
	distinct := func(a, b lipgloss.Style) bool { return a.GetForeground() != b.GetForeground() }
	if !distinct(stBanner, stTitle) {
		t.Error("the program name and the category headings are the same colour")
	}
	if !distinct(stMark, stTitle) {
		t.Error("the header glyph and the category headings are the same colour")
	}
	if !distinct(stMark, stBanner) {
		t.Error("the header glyph and the program name are the same colour")
	}
	if !distinct(stProcess, stTitle) {
		t.Error("the in-progress marker and the category headings are the same colour")
	}
	if !distinct(stProcess, stOK) {
		t.Error("the in-progress marker and a result are the same colour")
	}
}

// The banner is its own runs, so the glyph can be told apart from the name even
// where both are plain text.
func TestBannerRendersTheGlyphSeparately(t *testing.T) {
	restore := with(t, at(colorprofile.TrueColor, TierUnicode, 80))
	defer restore()
	var out strings.Builder
	swap := setWriter(&out, at(colorprofile.TrueColor, TierUnicode, 80))
	defer swap()
	Banner("launcher", "development")
	got := strings.TrimPrefix(out.String(), "\n")
	glyph := G(GlyphRocket)
	if !strings.HasPrefix(got, stMark.Render(glyph)) {
		t.Errorf("the header glyph must be its own run, got %q", got)
	}
	if !strings.Contains(got, stBanner.Render("launcher")) {
		t.Errorf("the program name must carry its own style, got %q", got)
	}
	if strings.Contains(got, stTitle.Render("launcher")) {
		t.Errorf("the program name must not be painted as a heading, got %q", got)
	}
}
