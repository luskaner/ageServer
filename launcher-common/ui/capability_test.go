package ui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// asciiProbe is the probe of a console that can do nothing at all. It is the only
// tier the parity guarantee has to hold for.
func asciiProbe() probe {
	return probe{
		isTerminal: func() bool { return true },
		vtUsable:   func() bool { return false },
		outputCode: func() uint32 { return 437 },
		columns:    func() int { return 80 },
		localeUTF8: func() bool { return false },
		osVersion:  func() (uint32, uint32) { return 6, 1 },
		isWindows:  func() bool { return true },
	}
}

// win10 is a modern Windows console with a working VT and the OEM code page.
func win10() probe {
	return probe{
		isTerminal: func() bool { return true },
		vtUsable:   func() bool { return true },
		outputCode: func() uint32 { return 437 },
		columns:    func() int { return 120 },
		localeUTF8: func() bool { return false },
		osVersion:  func() (uint32, uint32) { return 10, 19045 },
		isWindows:  func() bool { return true },
	}
}

func win10UTF8() probe {
	p := win10()
	p.outputCode = func() uint32 { return 65001 }
	return p
}

func unixProbe(localeUTF8 bool) probe {
	return probe{
		isTerminal: func() bool { return true },
		vtUsable:   func() bool { return true },
		outputCode: func() uint32 { return 0 },
		columns:    func() int { return 100 },
		localeUTF8: func() bool { return localeUTF8 },
		osVersion:  func() (uint32, uint32) { return 0, 0 },
		isWindows:  func() bool { return false },
	}
}

// win7VT pretends to be a Windows 7 console where VT happens to work, which is
// what ANSICON provides.
func win7VT() probe {
	p := asciiProbe()
	p.vtUsable = func() bool { return true }
	return p
}

// win7UTF8 is a Windows 7 console with an UTF-8 code page, i.e. after a
// `chcp 65001`. It must still stay in TierASCII: build 7601 has no VT support.
func win7UTF8() probe {
	p := win7VT()
	p.outputCode = func() uint32 { return 65001 }
	return p
}

// ttyEnv makes colorprofile see a terminal, which io.Discard never is. Without
// it every assertion below would pass for the wrong reason: NoTTY is what a pipe
// reports no matter what the rest of the probe says.
func ttyEnv(env ...string) []string {
	return append([]string{"TTY_FORCE=1"}, env...)
}

// TestDetectTierTable pins the tier of every terminal shape the design calls
// out. The colour profile is asserted separately, because colorprofile's answer
// depends on the host the test happens to run on.
func TestDetectTierTable(t *testing.T) {
	pipe := asciiProbe()
	pipe.isTerminal = func() bool { return false }
	win10NoVT := win10()
	win10NoVT.vtUsable = func() bool { return false }
	narrow := unixProbe(true)
	narrow.columns = func() int { return 40 }

	for _, tc := range []struct {
		name string
		env  []string
		p    probe
		tier GlyphTier
	}{
		// The case the whole design exists for.
		{"win7 cmd", nil, asciiProbe(), TierASCII},
		{"win7 ansicon", []string{"ANSICON=1"}, win7VT(), TierASCII},
		{"win7 chcp 65001", nil, win7UTF8(), TierASCII},
		// H1: colorprofile trusts a modern build without checking VT.
		{"win10 conhost without vt", nil, win10NoVT, TierASCII},
		{"win10 conhost with vt and cp437", nil, win10(), TierASCII},
		{"win10 with vt and cp65001", nil, win10UTF8(), TierUnicode},
		{"windows terminal", []string{"WT_SESSION=abc"}, win10UTF8(), TierEmoji},
		// Regression: Windows Terminal draws every glyph in its catalogue whatever
		// GetConsoleOutputCP reports, so a CP 437 answer must not strand a window
		// that can clearly do better on ASCII markers.
		{"windows terminal reporting cp437", []string{"WT_SESSION=abc"}, win10(), TierEmoji},
		{"windows terminal on an old build", []string{"WT_SESSION=abc"}, win7VT(), TierEmoji},
		{"conemu", []string{"ConEmuANSI=ON"}, win10UTF8(), TierEmoji},
		{"linux LANG=C", []string{"TERM=xterm-256color"}, unixProbe(false), TierASCII},
		{"linux utf8 locale", []string{"TERM=xterm-256color"}, unixProbe(true), TierEmoji},
		{"linux known good terminal without utf8 locale", []string{"TERM=alacritty"}, unixProbe(false), TierEmoji},
		{"linux utf8 locale on a narrow console", []string{"TERM=dumb-ish"}, narrow, TierUnicode},
		{"term dumb", []string{"TERM=dumb"}, unixProbe(true), TierASCII},
		// Not a console: this is the rule that keeps the golden tests green.
		{"pipe", nil, pipe, TierASCII},
		{"forced ascii", []string{"AGE_LANSERVER_OUTPUT=ascii", "WT_SESSION=abc"}, win10UTF8(), TierASCII},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(io.Discard, ttyEnv(tc.env...), tc.p); got.Tier != tc.tier {
				t.Errorf("Tier = %d, want %d", got.Tier, tc.tier)
			}
		})
	}
}

// TestDetectColourDecision pins the profile for every case this package decides
// itself, without depending on what colorprofile makes of the host.
func TestDetectColourDecision(t *testing.T) {
	pipe := asciiProbe()
	pipe.isTerminal = func() bool { return false }
	win10NoVT := win10()
	win10NoVT.vtUsable = func() bool { return false }

	for _, tc := range []struct {
		name string
		env  []string
		p    probe
		want colorprofile.Profile
	}{
		{"pipe", nil, pipe, colorprofile.NoTTY},
		{"forced ascii", []string{"AGE_LANSERVER_OUTPUT=ascii"}, win10UTF8(), colorprofile.NoTTY},
		{"term dumb", []string{"TERM=dumb"}, unixProbe(true), colorprofile.NoTTY},
		// H1.
		{"win10 conhost without vt", nil, win10NoVT, colorprofile.NoTTY},
		// H2: an empty NO_COLOR= still disables colour.
		{"NO_COLOR empty", []string{"TERM=xterm-256color", "NO_COLOR="}, unixProbe(true), colorprofile.NoTTY},
		{"NO_COLOR set", []string{"TERM=xterm-256color", "NO_COLOR=1"}, unixProbe(true), colorprofile.NoTTY},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(io.Discard, ttyEnv(tc.env...), tc.p); got.Profile != tc.want {
				t.Errorf("Profile = %v, want %v", got.Profile, tc.want)
			}
		})
	}
}

// An explicit request is the only way to keep colour with NO_COLOR set, and it
// still cannot conjure colour on a console that has none.
func TestForcedColorBeatsNoColor(t *testing.T) {
	got := Detect(io.Discard, ttyEnv("AGE_LANSERVER_OUTPUT=color", "TERM=xterm-256color", "NO_COLOR="), unixProbe(true))
	if got.Profile < colorprofile.ANSI {
		t.Fatalf("Profile = %v, want at least ANSI", got.Profile)
	}
	p := unixProbe(true)
	p.vtUsable = func() bool { return false }
	if got := Detect(io.Discard, ttyEnv("AGE_LANSERVER_OUTPUT=color", "TERM=xterm-256color"), p); got.Tier != TierASCII {
		t.Errorf("Tier = %d, want a forced colour request to degrade to TierASCII, not TierUnicode", got.Tier)
	}
}

// NO_COLOR is a request for plain output, so it caps the emoji as well as the
// colour: honouring it for colour while still printing a page of emoji is
// honouring it halfway. Unicode is kept, because those symbols come from the code
// page and cost one cell.
func TestNoColorCapsTheEmoji(t *testing.T) {
	got := Detect(io.Discard, ttyEnv("TERM=xterm-256color", "NO_COLOR="), unixProbe(true))
	if got.ColorOk() {
		t.Fatalf("an empty NO_COLOR must not leave colour on, got %v", got.Profile)
	}
	if got.Tier != TierUnicode {
		t.Errorf("Tier = %d, want TierUnicode (%d)", got.Tier, TierUnicode)
	}
}

// CI is a pipeline asking for simple symbols, not a person at a capable terminal.
func TestCICapsTheEmoji(t *testing.T) {
	for _, env := range [][]string{
		{"TERM=xterm-256color", "CI=true"},
		{"TERM=xterm-256color", "CI=1"},
		{"TERM=xterm-256color", "CI=anything-else"},
	} {
		if got := Detect(io.Discard, ttyEnv(env...), unixProbe(true)); got.Tier != TierUnicode {
			t.Errorf("%v: Tier = %d, want TierUnicode (%d)", env, got.Tier, TierUnicode)
		}
	}
	// An explicit false, and no variable at all, leave the emoji alone.
	for _, env := range [][]string{
		{"TERM=xterm-256color", "CI=false"},
		{"TERM=xterm-256color", "CI=0"},
		{"TERM=xterm-256color"},
		{"TERM=xterm-256color", "WT_SESSION=abc"},
	} {
		if got := Detect(io.Discard, ttyEnv(env...), unixProbe(true)); got.Tier != TierEmoji {
			t.Errorf("%v: Tier = %d, want TierEmoji (%d)", env, got.Tier, TierEmoji)
		}
	}
}

// TERM=dumb is the minimal terminal, and it was already the plain text fallback.
func TestTermDumbIsPlainText(t *testing.T) {
	got := Detect(io.Discard, ttyEnv("TERM=dumb"), unixProbe(true))
	if got.Tier != TierASCII {
		t.Errorf("Tier = %d, want TierASCII (%d)", got.Tier, TierASCII)
	}
	if got.ColorOk() {
		t.Errorf("TERM=dumb must not keep colour, got %v", got.Profile)
	}
}

func TestDetectIgnoresUnknownForcedOutput(t *testing.T) {
	// A typo must not lock the user into a degraded output with no obvious cause.
	env := ttyEnv("AGE_LANSERVER_OUTPUT=chartreuse")
	if got := Detect(io.Discard, env, win10UTF8()); got.Tier != TierUnicode {
		t.Fatalf("Tier = %d, want TierUnicode", got.Tier)
	}
}

func TestTermMatches(t *testing.T) {
	for _, tc := range []struct {
		term  string
		names []string
		want  bool
	}{
		{"xterm-256color", unicodeTerms, false},
		{"xterm", unicodeTerms, false},
		{"alacritty", unicodeTerms, true},
		{"kitty-direct", unicodeTerms, true},
		{"linux", unicodeTerms, false},
		{"", unicodeTerms, false},
		{"alacrittys", unicodeTerms, false},
	} {
		if got := termMatches(tc.term, tc.names); got != tc.want {
			t.Errorf("termMatches(%q) = %t, want %t", tc.term, got, tc.want)
		}
	}
}

func TestEnvPresentAndForcedOutput(t *testing.T) {
	env := []string{"A=1", "NO_COLOR=", "AGE_LANSERVER_OUTPUT=ASCII"}
	if !envPresent(env, "NO_COLOR") {
		t.Error("an empty NO_COLOR must count as present")
	}
	if envPresent(env, "NO_COLOUR") {
		t.Error("NO_COLOUR is not NO_COLOR")
	}
	if got := forcedOutput(env); got != "ascii" {
		t.Errorf("forcedOutput = %q, want ascii", got)
	}
	if got := forcedOutput([]string{"AGE_LANSERVER_OUTPUT=nonsense"}); got != "auto" {
		t.Errorf("forcedOutput = %q, want auto", got)
	}
}

func TestColumnLimitIsZeroWhenUnknown(t *testing.T) {
	p := asciiProbe()
	p.columns = func() int { return 0 }
	if got := Detect(io.Discard, nil, p).ColumnLimit(); got != 0 {
		t.Fatalf("ColumnLimit = %d, want 0 so nothing gets wrapped", got)
	}
}

// The hint only exists for the case it can fix: a modern Windows console that
// could do Unicode and was not told to.
func TestUpgradeHint(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   []string
		tier  GlyphTier
		probe probe
		want  string
	}{
		{"windows 10 on 437", nil, TierASCII, win10(), "437"},
		{"windows 10 on 65001", nil, TierUnicode, win10UTF8(), ""},
		{"windows 7", nil, TierASCII, asciiProbe(), ""},
		{"windows 7 with vt", nil, TierASCII, win7VT(), ""},
		{"no console", nil, TierASCII, withColumns(win10(), 0), ""},
		{"already unicode", nil, TierUnicode, win10(), ""},
		{"unix", nil, TierASCII, unixProbe(false), ""},
		// Windows Terminal gets the emoji tier whatever code page it reports, so
		// there is never anything to tell its user to fix.
		{"windows terminal", []string{"WT_SESSION=abc"}, TierEmoji, win10(), ""},
		{"windows terminal on ascii", []string{"WT_SESSION=abc"}, TierASCII, win10(), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := with(t, at(colorprofile.TrueColor, tc.tier, 80))
			defer restore()
			got := upgradeHint(ttyEnv(tc.env...), tc.probe)
			if tc.want == "" {
				if got != "" {
					t.Fatalf("got %q, want no hint", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) || !strings.Contains(got, "chcp 65001") {
				t.Errorf("got %q, want the code page and the command", got)
			}
		})
	}
}

func withColumns(p probe, columns int) probe {
	p.columns = func() int { return columns }
	return p
}
