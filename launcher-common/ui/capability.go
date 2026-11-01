// Package ui renders the console output of the launcher modules.
//
// It resolves what the destination terminal can actually display before
// printing anything, because "pretty" is not a boolean. Three independent axes
// decide the rendering: the colour profile (which SGR sequences survive), the
// glyph tier (which characters render without becoming replacement boxes) and
// the column count (whether anything must be wrapped at all).
//
// The hard guarantee is TierASCII: in that tier nothing outside 0x09, 0x0A-0x7E
// and printable ANSI is emitted, and every message keeps the exact wording it
// has today. A cmd.exe on Windows 7 (code page 437, no VT support) lands there,
// so it keeps showing at least the current output.
package ui

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

// GlyphTier is the most complex glyph the terminal can show without degrading
// into replacement characters.
type GlyphTier uint8

const (
	// TierASCII is printable ASCII only (0x20-0x7E). The one tier guaranteed on
	// a Windows 7 cmd.exe.
	TierASCII GlyphTier = iota
	// TierUnicode adds box drawing, arrows, geometric shapes and BMP dingbats.
	// Needs a UTF-8 code page on Windows and a font covering those blocks.
	TierUnicode
	// TierEmoji adds double width emoji. Needs everything above plus a font with
	// emoji, or a terminal known to be good at them.
	TierEmoji
)

// Capability is the immutable result of the detection.
type Capability struct {
	// Profile decides colour downsampling (NoTTY, ASCII, ANSI, ANSI256,
	// TrueColor).
	Profile colorprofile.Profile
	// Tier decides the most complex glyph allowed.
	Tier GlyphTier
	// Columns is the console width, or 0 when it could not be determined.
	Columns int
}

// ColorOk reports whether SGR sequences can be emitted safely.
func (c Capability) ColorOk() bool { return c.Profile >= colorprofile.ANSI }

// ColumnLimit returns the width to wrap to, or 0 when nothing should be
// wrapped. 0 is the right answer whenever the output is not a console, because
// inserting artificial line breaks into a file or a pipe only corrupts it.
func (c Capability) ColumnLimit() int { return c.Columns }

// probe groups the system queries so tests never touch the real console. Any nil
// field falls back to the production implementation.
type probe struct {
	isTerminal func() bool
	// vtUsable reports whether ANSI sequences are understood. On Windows that
	// means ENABLE_VIRTUAL_TERMINAL_PROCESSING verified by a re-read.
	vtUsable func() bool
	// outputCode is the Windows console output code page.
	outputCode func() uint32
	// columns is the console width, or 0.
	columns func() int
	// localeUTF8 reports a UTF-8 locale (Unix only).
	localeUTF8 func() bool
	// osVersion is the Windows NT version, as major/build.
	osVersion func() (major, build uint32)
	// isWindows selects which platform's glyph rules apply. The tier rules are
	// completely different on either side, so the test suite has to be able to
	// exercise both from one host.
	isWindows func() bool
}

// outputEnvVar is the environment override. "ascii" and "color" pin the
// decision, "auto" (or unset) leaves it to detection.
const outputEnvVar = "AGE_LANSERVER_OUTPUT"

// forcedOutput returns the override requested through the environment, if any.
// An unknown value is ignored so a typo cannot lock the user into a degraded
// output with no obvious cause.
func forcedOutput(env []string) string {
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if ok && name == outputEnvVar {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "ascii", "color", "auto":
				return strings.ToLower(strings.TrimSpace(value))
			}
		}
	}
	return "auto"
}

// envPresent reports whether a variable is present at all, regardless of its
// value. NO_COLOR uses this rule: per its spec the mere presence of the
// variable disables colour, so an empty NO_COLOR= counts. colorprofile instead
// runs the value through strconv.ParseBool, which treats "" as unset.
func envPresent(env []string, name string) bool {
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && k == name {
			return true
		}
	}
	return false
}

func (p probe) fill() probe {
	def := defaultProbe()
	if p.isTerminal == nil {
		p.isTerminal = def.isTerminal
	}
	if p.vtUsable == nil {
		p.vtUsable = def.vtUsable
	}
	if p.outputCode == nil {
		p.outputCode = def.outputCode
	}
	if p.columns == nil {
		p.columns = def.columns
	}
	if p.localeUTF8 == nil {
		p.localeUTF8 = def.localeUTF8
	}
	if p.osVersion == nil {
		p.osVersion = def.osVersion
	}
	if p.isWindows == nil {
		p.isWindows = def.isWindows
	}
	return p
}

func defaultProbe() probe {
	return probe{
		isTerminal: func() bool {
			return term.IsTerminal(os.Stdout.Fd())
		},
		vtUsable:   vtUsable,
		outputCode: outputCode,
		columns:    consoleColumns,
		localeUTF8: localeUTF8,
		osVersion:  osVersion,
		isWindows:  func() bool { return runtime.GOOS == "windows" },
	}
}

// fallback is the conservative answer: no colour, no glyphs beyond ASCII, no
// wrapping. Everything that cannot prove otherwise ends up here.
func fallback() Capability {
	return Capability{Profile: colorprofile.NoTTY, Tier: TierASCII}
}

// Detect is the only function that produces a Capability.
func Detect(w io.Writer, env []string, p probe) Capability {
	p = p.fill()
	forced := forcedOutput(env)
	if forced == "ascii" {
		return fallback()
	}
	// No console: a pipe, a file, or a go test. This rule is what keeps the
	// existing golden tests passing untouched, because in TierASCII the base
	// text is byte for byte what it is today.
	if !p.isTerminal() {
		return fallback()
	}
	profile := colorprofile.Detect(w, env)
	// H2: presence, not ParseBool. An explicit "color" request is the only way
	// to keep colour with NO_COLOR set.
	noColor := envPresent(env, "NO_COLOR")
	if noColor && forced != "color" {
		profile = colorprofile.NoTTY
	}
	// A NoTTY profile that is not caused by NO_COLOR means the terminal cannot
	// do colour at all (TERM=dumb, Windows older than 10586). It says nothing
	// about glyphs on its own, but there is no good reason to emit fancy
	// characters to a terminal we already know is minimal.
	if profile == colorprofile.NoTTY && !noColor {
		return fallback()
	}
	// H1: colorprofile trusts build >= 14931 without checking that the console
	// actually has VT enabled, which prints the escape sequences literally.
	if profile >= colorprofile.ANSI && !p.vtUsable() {
		return fallback()
	}
	c := Capability{Profile: profile, Columns: p.columns()}
	if c.Columns < 0 {
		c.Columns = 0
	}
	c.Tier = detectTier(env, p, c.Columns)
	c.Tier = capEmojiTier(c.Tier, env)
	return c
}

// capEmojiTier is the last word on the glyph tier, and it comes from the
// environment rather than from the terminal.
//
// Emoji are the one thing in the catalogue that needs a font rather than a code
// page, and there are two cases where even a capable terminal should not be asked
// to draw them:
//
//   - NO_COLOR, which is a request for plain output. Honouring it for colour while
//     still printing a page of emoji is honouring it halfway.
//   - CI, which is a request from a pipeline rather than from a person: a build log
//     read in a web page is not served the font that draws a blue diamond.
//
// Both cap the tier at Unicode rather than dropping it to ASCII. Unicode symbols
// are single cell, come from the code page, and are exactly the "simple symbols"
// that a log asks for, so the degradation stays as small as it can.
func capEmojiTier(tier GlyphTier, env []string) GlyphTier {
	if tier != TierEmoji {
		return tier
	}
	if envPresent(env, "NO_COLOR") || envTruthy(env, "CI") {
		return TierUnicode
	}
	return tier
}

// envTruthy reports whether a variable is set and does not say false.
//
// A value that does not parse as a boolean counts as true, because every CI
// provider that sets the variable means it, and an explicit "false" or "0" is
// respected so a shell that exports CI=false is not overruled.
func envTruthy(env []string, name string) bool {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k != name {
			continue
		}
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
		return true
	}
	return false
}

// detectTier decides the glyph tier for a console already known to be usable.
func detectTier(env []string, p probe, columns int) GlyphTier {
	if p.isWindows() {
		return detectTierWindows(env, p)
	}
	// Unix: a UTF-8 locale is enough for the BMP, and a terminal known to
	// render it is enough even without one.
	if !p.localeUTF8() && !termKnownUnicode(env) {
		return TierASCII
	}
	if termKnownEmoji(env) || (p.localeUTF8() && columns >= minEmojiColumns) {
		return TierEmoji
	}
	return TierUnicode
}

// detectTierWindows requires all three of a VT-capable modern Windows, a UTF-8
// output code page and a usable VT. Asking for all three is deliberate: a
// `chcp 65001` on Windows 7 must not raise the tier, and neither must a Windows
// 10 cmd without the chcp.
//
// Windows Terminal is the one exception, and it is not a guess: it is a Unicode
// terminal that emulates a console, so it draws emoji, box drawing and everything
// else in its catalogue whatever GetConsoleOutputCP happens to report. A ConPTY
// client sees the code page the last client set, not the one the window renders
// with, so on Windows Terminal the code page is worth nothing and following it
// leaves a window full of capable glyphs stuck on ASCII markers.
func detectTierWindows(env []string, p probe) GlyphTier {
	// Windows Terminal ships its own VT interpreter and its own font: no build
	// check, no code page, no probe. A terminal that emulates a console this well
	// does not need to be asked what it can do.
	if envPresent(env, "WT_SESSION") {
		return TierEmoji
	}
	major, build := p.osVersion()
	if major < 10 || build < 10586 || !p.vtUsable() || p.outputCode() != utf8CodePage {
		return TierASCII
	}
	// ConEmu brings its own VT interpreter and font too, but its code page is a
	// setting rather than a property, so it still has to be checked.
	if envValue(env, "ConEmuANSI") == "ON" {
		return TierEmoji
	}
	return TierUnicode
}

// utf8CodePage is the Windows code page for UTF-8.
const utf8CodePage = 65001

// isWindows picks the platform behaviour that cannot be decided at build time by
// this file alone. It is a var and not a constant so a test can pretend to be on
// the other one; nothing else may assign it.
var isWindows = runtime.GOOS == "windows"

// minEmojiColumns is the narrowest console where double width markers still
// leave room for a readable message. Below it, emoji would eat a quarter of
// the line.
const minEmojiColumns = 60

// unicodeTerms are the terminals whose bundled fonts are known to cover the BMP
// symbols the glyph catalogue uses. A VT-emulating terminal with unknown fonts
// (xterm and friends) is deliberately absent: there, the locale decides.
var unicodeTerms = []string{
	"alacritty", "kitty", "wezterm", "foot", "ghostty", "rio", "vte", "konsole", "st",
}

// emojiTerms are the terminals known to render emoji as expected.
var emojiTerms = []string{
	"kitty", "alacritty", "wezterm", "foot", "ghostty", "rio", "konsole",
}

func envValue(env []string, name string) string {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v
		}
	}
	return ""
}

// termMatches reports whether term is one of names, or a variant of one of them
// ("xterm-256color" matches "xterm").
func termMatches(term string, names []string) bool {
	if term == "" {
		return false
	}
	for _, name := range names {
		if term == name || strings.HasPrefix(term, name+"-") || strings.HasPrefix(term, name+".") {
			return true
		}
	}
	return false
}

func termKnownUnicode(env []string) bool {
	return termMatches(envValue(env, "TERM"), unicodeTerms) ||
		envValue(env, "COLORTERM") != "" ||
		envPresent(env, "WT_SESSION")
}

func termKnownEmoji(env []string) bool {
	if termMatches(envValue(env, "TERM"), emojiTerms) {
		return true
	}
	// A bare "xterm" is too old a guess to promise emoji on, but a recent one
	// is close enough that a wide console plus a UTF-8 locale seals it.
	return strings.HasPrefix(envValue(env, "TERM"), "xterm-") && envValue(env, "COLORTERM") != ""
}

// UpgradeHint returns a one line explanation of how to get better glyphs, or an
// empty string when there is nothing to upgrade to.
//
// It exists because the ASCII tier is not a preference, it is a code page: a
// Windows console left on 437 turns a UTF-8 tick into three unrelated characters,
// so the markers degrade to emoticons and the reader is left guessing whether
// that is what the program meant to print. Naming the one command that fixes it
// is more useful than the fallback itself.
func UpgradeHint() string { return upgradeHint(os.Environ(), probe{}.fill()) }

// upgradeHint is UpgradeHint with injectable inputs, so the decision can be
// tested on any host.
func upgradeHint(env []string, p probe) string {
	if Current().Tier != TierASCII {
		return ""
	}
	p = p.fill()
	// Windows Terminal already gets the best tier, so there is never anything to
	// tell its user, and its reported code page is not something a chcp changes.
	if envPresent(env, "WT_SESSION") {
		return ""
	}
	if !p.isWindows() {
		return ""
	}
	// Windows 7 has no Unicode console worth switching to, and no VT either: it
	// is exactly the case the fallback exists for, so it never gets a hint it
	// cannot act on.
	major, build := p.osVersion()
	if major < 10 || build < 10586 {
		return ""
	}
	// A terminal that is not a console at all (a pipe, a file) is already being
	// written in plain text on purpose.
	if p.columns() <= 0 {
		return ""
	}
	code := p.outputCode()
	if code == utf8CodePage || !p.vtUsable() {
		return ""
	}
	// A code page of zero means the query failed, which says nothing about what the
	// console can draw. The command is still the right advice, so it is given
	// without the number rather than with a wrong one.
	if code == 0 {
		return "Run 'chcp 65001' (or start the launcher through start.bat) for ✓ ✗ ▲ ● and colour."
	}
	return "This console uses code page " + strconv.FormatUint(uint64(code), 10) +
		". Run 'chcp 65001' (or start the launcher through start.bat) for ✓ ✗ ▲ ● and colour."
}

// current holds the resolved capability and the console destination.
//
// console stays nil until Initialize or SetOutput is called, and nil means
// "whatever os.Stdout is right now". Caching os.Stdout in a variable at init
// time would freeze it, and every test in this repo that swaps os.Stdout for a
// pipe to capture the console output would then capture nothing.
var (
	currentMu sync.RWMutex
	current   = fallback()
	console   io.Writer
)

// Current returns the resolved Capability.
func Current() Capability {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current
}

// Writer returns the console destination.
func Writer() io.Writer {
	currentMu.RLock()
	w := console
	currentMu.RUnlock()
	if w == nil {
		return os.Stdout
	}
	return w
}

func set(w io.Writer, c Capability) {
	currentMu.Lock()
	console = w
	current = c
	currentMu.Unlock()
	initTheme(c)
}

// Initialize fixes the console destination and computes the Capability. Call it
// once from main(), before commonLogger.Initialize, so nothing else has to know
// about the detection. A nil writer keeps following os.Stdout.
func Initialize(w io.Writer, env []string) {
	// The code page has to be settled before anything is detected: the glyph tier
	// is decided from it, and a console left on a legacy code page cannot draw the
	// catalogue no matter what the terminal itself is capable of. Every program
	// that prints goes through here, so each one puts its own console on UTF-8
	// rather than leaving it to a wrapper script.
	if forcedOutput(env) != "ascii" {
		ensureUTF8Output()
	}
	set(w, Detect(Writer(), env, probe{}))
}

// SetOutput changes the console destination and returns the restore function,
// following the convention already used by common.SetTerminal and
// dialog.SetOutput. Intended for tests.
func SetOutput(w io.Writer) (restore func()) {
	currentMu.RLock()
	prevOut, prevCap := console, current
	currentMu.RUnlock()
	set(w, prevCap)
	return func() { set(prevOut, prevCap) }
}

// SetCapability overrides the resolved Capability and returns the function that
// puts the previous one back. It is the sibling of SetOutput for the other axis,
// and exists so a test can pin the tier its assertions depend on: go test always
// runs with a pipe, which means the automatic answer is TierASCII and the
// Unicode and emoji branches would otherwise never be exercised.
func SetCapability(c Capability) (restore func()) {
	currentMu.RLock()
	prevOut, prevCap := console, current
	currentMu.RUnlock()
	set(prevOut, c)
	return func() { set(prevOut, prevCap) }
}

// ApplyOverride re-runs detection honouring an explicit --output value, which
// is how the flag gets the last word over the environment. An unknown or empty
// value is ignored.
func ApplyOverride(value string) {
	currentMu.RLock()
	w := console
	currentMu.RUnlock()
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		set(w, Detect(Writer(), os.Environ(), probe{}))
	case "ascii":
		set(w, fallback())
	case "color":
		env := append(os.Environ(), outputEnvVar+"=color")
		set(w, Detect(Writer(), env, probe{}))
	}
}
