package uuid

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) UUID {
	t.Helper()
	u, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", s, err)
	}
	return u
}

func TestParseCanonical(t *testing.T) {
	const s = "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"
	const wantHex = "f81d4fae7dec11d0a76500a0c91e6bf6"
	got := mustParse(t, s)
	if gotHex := hex.EncodeToString(got[:]); gotHex != wantHex {
		t.Fatalf("Parse canonical mismatch: got %x (%v), want %s", got[:], got, wantHex)
	}
	if got.String() != s {
		t.Fatalf("String() = %q, want %q", got.String(), s)
	}
}

func TestParseAcceptedForms(t *testing.T) {
	canon := mustParse(t, "f81d4fae-7dec-11d0-a765-00a0c91e6bf6")
	for _, form := range []string{
		"{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}",
		"urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6",
		"f81d4fae7dec11d0a76500a0c91e6bf6",
		"F81D4FAE-7DEC-11D0-A765-00A0C91E6BF6",
	} {
		if got := mustParse(t, form); got != canon {
			t.Errorf("Parse(%q) = %v, want %v", form, got, canon)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, s := range []string{
		"",
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf",
		"f81d4faegdec-11d0-a765-00a0c91e6bf6",
		"zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
		"f81d4fae+7dec-11d0-a765-00a0c91e6bf6",
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", s)
		}
	}
}

func TestUnmarshalTextErrorKeepsReceiver(t *testing.T) {
	u := Max()
	before := u
	if err := u.UnmarshalText([]byte("nope")); err == nil {
		t.Fatal("expected error")
	}
	if u != before {
		t.Fatalf("receiver modified on error: %v != %v", u, before)
	}
}

func TestNilAndMax(t *testing.T) {
	if Nil().String() != "00000000-0000-0000-0000-000000000000" {
		t.Fatal("unexpected Nil string")
	}
	if Max().String() != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatal("unexpected Max string")
	}
}

func TestCompare(t *testing.T) {
	low := mustParse(t, "00000000-0000-0000-0000-000000000001")
	high := mustParse(t, "00000000-0000-0000-0000-000000000002")
	if c := low.Compare(high); c != -1 {
		t.Errorf("low.Compare(high) = %d, want -1", c)
	}
	if c := high.Compare(low); c != 1 {
		t.Errorf("high.Compare(low) = %d, want 1", c)
	}
	if c := low.Compare(low); c != 0 {
		t.Errorf("low.Compare(low) = %d, want 0", c)
	}
}

func TestAppendText(t *testing.T) {
	u := mustParse(t, "f81d4fae-7dec-11d0-a765-00a0c91e6bf6")

	// Appends after existing content.
	b, err := u.AppendText([]byte("id="))
	if err != nil {
		t.Fatalf("AppendText error: %v", err)
	}
	if want := "id=f81d4fae-7dec-11d0-a765-00a0c91e6bf6"; string(b) != want {
		t.Fatalf("AppendText = %q, want %q", b, want)
	}

	// Pre-existing content is preserved before the appended representation.
	b2, err := u.AppendText([]byte("head;"))
	if err != nil {
		t.Fatalf("AppendText error: %v", err)
	}
	if !strings.HasPrefix(string(b2), "head;f81d4fae") {
		t.Fatalf("AppendText = %q, existing content not preserved", b2)
	}
}

func TestMarshalTextMatchesString(t *testing.T) {
	u := New()
	mt, err := u.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}
	if string(mt) != u.String() {
		t.Fatalf("MarshalText %q != String %q", mt, u.String())
	}
}

func TestNewV4DeterministicAndBits(t *testing.T) {
	src := bytes.NewReader(bytes.Repeat([]byte{0xAB}, 64))
	SetRand(src)
	defer SetRand(nil)

	u := NewV4()
	// 0xAB everywhere except: byte 6 low nibble forced to version 4 (0x4B),
	// byte 8 top bits forced to variant 10 (0xAB already satisfies it).
	want := bytes.Repeat([]byte{0xAB}, 16)
	want[6] = 0x4B
	if !bytes.Equal(u[:], want) {
		t.Fatalf("NewV4 = %x, want %x", u[:], want)
	}
	if v := u[6] >> 4; v != 4 {
		t.Fatalf("version nibble = %d, want 4", v)
	}
	if variant := u[8] >> 6; variant != 0b10 {
		t.Fatalf("variant bits = %02b, want 10", variant)
	}

	u2 := NewV4()
	if u2 != u {
		t.Fatal("expected deterministic stream to repeat with same seed state")
	}
}

func TestSetRandNilRestoresCryptoReader(t *testing.T) {
	SetRand(nil)
	a := New()
	b := New()
	if a == b {
		t.Fatal("two New() calls after restoring random source produced identical UUIDs")
	}
}

func TestMustParsePanicsOnInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustParse did not panic on invalid input")
		}
	}()
	MustParse("invalid")
}

func TestUnmarshalTextHexDecodeBranches(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"group1 bad hex", "xxxxxxxx-7dec-11d0-a765-00a0c91e6bf6"},
		{"group2 bad hex", "f81d4fae-xxxx-11d0-a765-00a0c91e6bf6"},
		{"group3 bad hex", "f81d4fae-7dec-xxxx-a765-00a0c91e6bf6"},
		{"group4 bad hex", "f81d4fae-7dec-11d0-xxxx-00a0c91e6bf6"},
		{"group5 bad hex", "f81d4fae-7dec-11d0-a765-xxxxxxxxxxxx"},
		{"length 45 (urn+37)", "urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6x"},
		{"length 33 (no-dashes)", "f81d4fae7dec11d0a76500a0c91e6bf6x"},
		{"length 38 (braced+extra)", "{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}xx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u UUID
			if err := u.UnmarshalText([]byte(tt.input)); err == nil {
				t.Errorf("UnmarshalText(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestAppendTextNilSlice(t *testing.T) {
	u := Nil()
	b, err := u.AppendText(nil)
	if err != nil {
		t.Fatalf("AppendText(nil): %v", err)
	}
	if string(b) != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("AppendText(nil) = %q", string(b))
	}
}

func TestUnmarshalText32CharHexInvalid(t *testing.T) {
	// 32-char hex string with invalid characters should fail
	var u UUID
	if err := u.UnmarshalText([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")); err == nil {
		t.Error("expected error for 32-char invalid hex")
	}
}
