package ui

import (
	"bytes"
	"io"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// with installs a Capability for the duration of the test and returns the
// function that puts the previous one back.
func with(t *testing.T, c Capability) (restore func()) {
	t.Helper()
	currentMu.RLock()
	prevOut, prevCap := console, current
	currentMu.RUnlock()
	t.Cleanup(func() { set(prevOut, prevCap) })
	set(new(bytes.Buffer), c)
	return func() { set(prevOut, prevCap) }
}

// withTier installs a plain capability at the given tier, which is what most of
// the rendering tests need: glyphs differ, colours do not.
func withTier(t *testing.T, tier GlyphTier) {
	t.Helper()
	with(t, Capability{Profile: colorprofile.NoTTY, Tier: tier})
}

// withColor installs a capability with colour enabled at the given tier.
func withColor(t *testing.T, tier GlyphTier) {
	t.Helper()
	with(t, Capability{Profile: colorprofile.TrueColor, Tier: tier})
}

// render runs fn with the capability installed and returns what reached the
// console.
func render(t *testing.T, c Capability, fn func()) string {
	t.Helper()
	restore := with(t, c)
	defer restore()
	buf, ok := Writer().(*bytes.Buffer)
	if !ok {
		t.Fatal("the console destination is not a buffer")
	}
	fn()
	return buf.String()
}

// setWriter installs a console destination and a Capability, returning the
// function that puts both back.
func setWriter(w io.Writer, c Capability) (restore func()) {
	currentMu.RLock()
	prevOut, prevCap := console, current
	currentMu.RUnlock()
	set(w, c)
	return func() { set(prevOut, prevCap) }
}

// at builds a Capability with an explicit profile, tier and width.
func at(profile colorprofile.Profile, tier GlyphTier, columns int) Capability {
	return Capability{Profile: profile, Tier: tier, Columns: columns}
}
