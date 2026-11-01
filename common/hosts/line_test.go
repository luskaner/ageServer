package hosts

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestParseLineMapping(t *testing.T) {
	ok, overLimit, l := ParseLine("1.2.3.4\texample.com", true)
	if !ok || overLimit {
		t.Fatalf("ok=%v overLimit=%v", ok, overLimit)
	}
	if !l.IP().Equal(net.ParseIP("1.2.3.4")) {
		t.Fatalf("IP = %v", l.IP())
	}
	if hosts := l.Hosts(); len(hosts) != 1 || hosts[0] != Host("example.com") {
		t.Fatalf("Hosts = %v", hosts)
	}
	if l.OnlyComments() {
		t.Fatal("mapping line must not be OnlyComments")
	}
}

func TestParseLineMultipleHosts(t *testing.T) {
	ok, _, l := ParseLine("10.0.0.1 a.test b.test c.test", true)
	if !ok {
		t.Fatal("expected ok")
	}
	if got := len(l.Hosts()); got != 3 {
		t.Fatalf("hosts = %d, want 3", got)
	}
}

func TestParseLineCommentOnly(t *testing.T) {
	ok, _, l := ParseLine("# just a comment", true)
	if !ok {
		t.Fatal("comment line should parse ok")
	}
	if !l.OnlyComments() {
		t.Fatal("expected OnlyComments")
	}
}

// Regression: Uncommented on a blank line used to panic with index out of range.
func TestBlankLineUncommentedNoPanic(t *testing.T) {
	ok, _, l := ParseLine("", true)
	if !ok || !l.OnlyComments() {
		t.Fatalf("blank line: ok=%v onlyComments=%v", ok, l.OnlyComments())
	}
	if got := l.Uncommented(); got != "" {
		t.Fatalf("Uncommented = %q, want empty", got)
	}
}

func TestParseLineInvalid(t *testing.T) {
	for _, line := range []string{
		"1.2.3.4",       // ip without host
		"notanip host",  // invalid ip
		"1.2.3.4 _priv", // underscore host rejected by IDNA lookup profile
	} {
		if ok, _, _ := ParseLine(line, true); ok {
			t.Errorf("line %q expected not-ok", line)
		}
	}
}

func TestParseLineOverLimit(t *testing.T) {
	// Unix limits line length (256); Windows limits hosts per line (9).
	switch {
	case maxCharsPerLine < 1<<20:
		long := strings.Repeat("a", maxCharsPerLine-len(LineEnding)+1)
		ok, overLimit, l := ParseLine("1.2.3.4 "+long, false)
		if !ok || !overLimit {
			t.Fatalf("ok=%v overLimit=%v", ok, overLimit)
		}
		if len(l.Hosts()) == 0 {
			t.Fatal("expected at least one host to be kept")
		}
	case maxHostsPerLine < 100:
		hosts := make([]string, maxHostsPerLine+2)
		for i := range hosts {
			hosts[i] = fmt.Sprintf("h%d.test", i)
		}
		ok, overLimit, l := ParseLine("1.2.3.4 "+strings.Join(hosts, " "), false)
		if !ok || !overLimit {
			t.Fatalf("ok=%v overLimit=%v", ok, overLimit)
		}
		if len(l.Hosts()) != maxHostsPerLine {
			t.Fatalf("kept hosts = %d, want %d", len(l.Hosts()), maxHostsPerLine)
		}
	default:
		t.Skip("no practical per-line limits on this platform")
	}
}

func TestCommentedRoundTrip(t *testing.T) {
	_, _, original := ParseLine("1.2.3.4 example.com", true)
	commented, nl := original.Commented()
	if !commented {
		t.Fatal("expected commented ok")
	}
	if s := nl.String(); !strings.HasPrefix(s, "#") {
		t.Fatalf("commented line = %q", s)
	}
	if !nl.OnlyComments() {
		t.Fatal("commented mapping line must be comments-only")
	}
	back := nl.Uncommented()
	if back != original.String() {
		t.Fatalf("Uncommented(Commented(line)) = %q, want %q", back, original.String())
	}
}

func TestOwnMarkingRoundTrip(t *testing.T) {
	_, _, l := ParseLine("1.2.3.4 example.com", true)
	if l.Own() {
		t.Fatal("fresh mapping must not be own-marked")
	}
	marked := l.WithOwnMarking()
	if !marked.Own() {
		t.Fatal("WithOwnMarking must set Own")
	}
	unmarked := marked.WithoutOwnMarking()
	if unmarked.Own() {
		t.Fatal("WithoutOwnMarking must clear Own")
	}
	// Idempotence
	if again := marked.WithOwnMarking(); again.Own() != true || len(again.comments) != len(marked.comments) {
		t.Fatal("WithOwnMarking must be idempotent")
	}
}

func TestLineStringFormat(t *testing.T) {
	_, _, l := ParseLine("1.2.3.4\ta.test b.test # trailing note", true)
	s := l.String()
	if !strings.HasPrefix(s, "1.2.3.4\t") {
		t.Fatalf("String = %q, want IP followed by tab", s)
	}
	if !strings.Contains(s, "a.test") || !strings.Contains(s, "b.test") {
		t.Fatalf("String lost hosts: %q", s)
	}
	if strings.Contains(s, "# trailing note") == false && !l.Own() {
		t.Fatalf("comment must be preserved: %q", s)
	}
}

func TestHostMappingsCRUD(t *testing.T) {
	m := HostMappings{}
	ip := net.ParseIP("127.0.0.1")
	m.Set(Host("a.test"), ip)
	got, ok := m.Get(Host("a.test"))
	if !ok || !got.Equal(ip) {
		t.Fatalf("Get after Set = %v, %v", got, ok)
	}
	m.Delete(Host("a.test"))
	if _, ok := m.Get(Host("a.test")); ok {
		t.Fatal("Delete did not remove entry")
	}
}

func TestHostMappingsString(t *testing.T) {
	m := HostMappings{}
	m.Set(Host("a.test"), net.ParseIP("127.0.0.2"))
	out := m.String("\n")
	if !strings.Contains(out, "127.0.0.2\ta.test") {
		t.Fatalf("String output missing mapping: %q", out)
	}
	if !strings.Contains(out, marking) {
		t.Fatalf("generated mappings must carry own marking: %q", out)
	}
	if !strings.HasPrefix(out, "\n") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("String must start and end with the given line ending: %q", out)
	}
}

func TestParseHostRejectsIPAsHost(t *testing.T) {
	if ok, _ := parseHost("1.2.3.4"); ok {
		t.Fatal("IP must not be accepted as host name")
	}
	if ok, parsed := parseHost("example.com"); !ok || parsed != Host("example.com") {
		t.Fatalf("parseHost(example.com) = %v, %v", ok, parsed)
	}
}

func TestParseLineOverLimitCharsMocked(t *testing.T) {
	origChars, origHosts := maxCharsPerLine, maxHostsPerLine
	defer func() { maxCharsPerLine, maxHostsPerLine = origChars, origHosts }()
	maxCharsPerLine = 20
	maxHostsPerLine = 100
	long := "1.2.3.4 " + strings.Repeat("a", 30) + " example.com"
	ok, overLimit, _ := ParseLine(long, false)
	if !ok || !overLimit {
		t.Fatalf("expected overLimit for long line, ok=%v overLimit=%v", ok, overLimit)
	}
}

func TestParseLineOverLimitHostsMocked(t *testing.T) {
	origChars, origHosts := maxCharsPerLine, maxHostsPerLine
	defer func() { maxCharsPerLine, maxHostsPerLine = origChars, origHosts }()
	maxCharsPerLine = 1000
	maxHostsPerLine = 2
	hosts := []string{"h1.test", "h2.test", "h3.test", "h4.test"}
	ok, overLimit, l := ParseLine("1.2.3.4 "+strings.Join(hosts, " "), false)
	if !ok || !overLimit {
		t.Fatalf("expected overLimit for many hosts, ok=%v overLimit=%v", ok, overLimit)
	}
	if len(l.Hosts()) != 2 {
		t.Fatalf("hosts kept = %d, want 2", len(l.Hosts()))
	}
}
