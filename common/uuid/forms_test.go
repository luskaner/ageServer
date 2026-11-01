package uuid

import (
	"testing"
)

// Branch matrix for UnmarshalText/Parse covering every accepted and rejected
// textual form.
func TestUnmarshalTextForms(t *testing.T) {
	canon := "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"
	want := mustParse(t, canon)

	valid := map[string]string{
		"canonical": canon,
		"braced":    "{" + canon + "}",
		"urn":       "urn:uuid:" + canon,
		"no-dashes": "f81d4fae7dec11d0a76500a0c91e6bf6",
		"uppercase": "F81D4FAE-7DEC-11D0-A765-00A0C91E6BF6",
	}
	for name, form := range valid {
		var got UUID
		if err := got.UnmarshalText([]byte(form)); err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestUnmarshalTextRejections(t *testing.T) {
	canon := "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"
	invalid := []string{
		"",
		canon + "x",                             // too long
		canon[:35],                              // too short
		"f81d4fae_7dec-11d0-a765-00a0c91e6bf6",  // bad separator
		"f81d4fae-7dec-11d0-a765-00a0c91e6bf",   // truncated hex
		"gg1d4fae-7dec-11d0-a765-00a0c91e6bf6",  // non-hex
		"{f81d4fae-7dec-11d0-a765-00a0c91e6bf6", // unbalanced brace
		"urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf", // urn + truncated
		"urn:uuid:{" + canon + "}",                     // combined urn+braces (rejected, same as stdlib)
	}
	for _, form := range invalid {
		var got UUID
		if err := got.UnmarshalText([]byte(form)); err == nil {
			t.Errorf("form %q unexpectedly accepted", form)
		}
	}
}

func TestParseRoundTripRandom(t *testing.T) {
	for i := 0; i < 32; i++ {
		u := New()
		parsed := MustParse(u.String())
		if parsed != u {
			t.Fatalf("round trip %d failed: %v != %v", i, parsed, u)
		}
	}
}
