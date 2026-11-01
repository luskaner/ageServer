package hosts

import (
	"strings"
	"testing"
)

func TestCommentedOnCommentsOnlyIsIdentity(t *testing.T) {
	ok, _, l := ParseLine("# already a comment", true)
	if !ok {
		t.Fatal("parse failed")
	}
	again, nl := l.Commented()
	if !again || nl.String() != l.String() {
		t.Fatalf("Comented on comments-only must be identity: %q vs %q", nl.String(), l.String())
	}
}

func TestUncommentedJoinsMultipleSegments(t *testing.T) {
	ok, _, l := ParseLine("# part1 # part2", true)
	if !ok {
		t.Fatal("parse failed")
	}
	got := l.Uncommented()
	if !strings.Contains(got, "part1") || !strings.Contains(got, "part2") {
		t.Fatalf("Uncommented = %q, want both segments joined", got)
	}
	if strings.HasPrefix(got, "#") {
		t.Fatalf("Uncommented must strip leading marker: %q", got)
	}
}

func TestHostStringAsciiPassthrough(t *testing.T) {
	h := Host("plain.example.test")
	if h.String() != "plain.example.test" {
		t.Fatalf("Host.String = %q", h.String())
	}
}

// IDNA: non-ASCII hosts are converted to their punycode form.
func TestParseHostUnicodeConvertsToPunycode(t *testing.T) {
	ok, parsed := parseHost("exämple.test")
	if !ok {
		t.Fatal("IDNA host rejected")
	}
	if ascii := parsed.String(); !strings.HasPrefix(ascii, "xn--") {
		t.Fatalf("punycode = %q, want xn-- prefix", ascii)
	}
}
