package logEntry

import (
	"crypto/sha512"
	"net"
	"testing"
	"time"
)

type stubEntry struct {
	uptime   time.Duration
	replayed *[]int
	checked  *[]int
	id       int
}

func (s *stubEntry) Uptime() time.Duration { return s.uptime }

func (s *stubEntry) String() string { return "stub" }

func (s *stubEntry) Replay(net.IP) {
	if s.replayed != nil {
		*s.replayed = append(*s.replayed, s.id)
	}
}

func (s *stubEntry) CheckResponse() {
	if s.checked != nil {
		*s.checked = append(*s.checked, s.id)
	}
}

func resetEntries() { entries = nil }

// Regression: Replay indexed entries[0] and panicked when the decoded log had
// no entries (e.g. an empty server communication log).
func TestReplayWithoutEntriesDoesNotPanic(t *testing.T) {
	resetEntries()
	defer resetEntries()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	Replay(net.ParseIP("127.0.0.1"), true)
}

func TestReplayRunsInUptimeOrderAndChecksAfterwards(t *testing.T) {
	resetEntries()
	defer resetEntries()

	var replayed, checked []int
	Add(&stubEntry{id: 3, uptime: 300, replayed: &replayed, checked: &checked})
	Add(&stubEntry{id: 1, uptime: 100, replayed: &replayed, checked: &checked})
	Add(&stubEntry{id: 2, uptime: 200, replayed: &replayed, checked: &checked})
	// Sorting is the decoder's job (Decode -> Sort -> Replay).
	Sort()

	Replay(net.ParseIP("127.0.0.1"), true)

	if len(replayed) != 3 || replayed[0] != 1 || replayed[1] != 2 || replayed[2] != 3 {
		t.Fatalf("replay order = %v", replayed)
	}
	if len(checked) != 3 || checked[0] != 1 || checked[1] != 2 || checked[2] != 3 {
		t.Fatalf("check order = %v", checked)
	}
}

func TestSameBody(t *testing.T) {
	body := []byte(`{"a":1}`)
	hash := sha512.Sum512(body)
	if !SameBody(hash, body) {
		t.Fatal("matching hash rejected")
	}
	if SameBody([64]byte{1}, body) {
		t.Fatal("non-matching hash accepted")
	}
	if !SameBody([64]byte{}, nil) {
		t.Fatal("empty body must match the zero hash")
	}
}

func TestMatchAllTrue(t *testing.T) {
	if !matchAll(func(s string) bool { return s == "a" }, "a", "a") {
		t.Fatal("matchAll should return true when all match")
	}
}

func TestMatchAllFalse(t *testing.T) {
	if matchAll(func(s string) bool { return s == "a" }, "a", "b") {
		t.Fatal("matchAll should return false when one doesn't match")
	}
}

func TestMatchAllTypeMismatch(t *testing.T) {
	if matchAll(func(s string) bool { return true }, 123) {
		t.Fatal("matchAll should return false on type mismatch")
	}
}

func TestMatchAllEmpty(t *testing.T) {
	if !matchAll(func(s string) bool { return false }) {
		t.Fatal("matchAll with no args should return true")
	}
}

func TestIpValid(t *testing.T) {
	if !ip("192.168.1.1") {
		t.Fatal("valid IPv4 rejected")
	}
	if !ip("2001:db8::1") {
		t.Fatal("valid IPv6 rejected")
	}
}

func TestIpInvalid(t *testing.T) {
	if ip("not-an-ip") {
		t.Fatal("invalid IP accepted")
	}
}

func TestEpochValid(t *testing.T) {
	now := time.Now()
	if !epoch(float64(now.Unix())) {
		t.Fatal("current time epoch rejected")
	}
}

func TestEpochTooOld(t *testing.T) {
	if epoch(float64(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix())) {
		t.Fatal("epoch before 2025 should be rejected")
	}
}

func TestEpochTooFarFuture(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	if epoch(float64(future.Unix())) {
		t.Fatal("epoch >24h in future should be rejected")
	}
}

func TestDateIso8601Valid(t *testing.T) {
	if !dateIso8601("2025-06-15T12:30:00.000Z") {
		t.Fatal("valid ISO 8601 rejected")
	}
}

func TestDateIso8601Invalid(t *testing.T) {
	if dateIso8601("not-a-date") {
		t.Fatal("invalid ISO 8601 accepted")
	}
}

func TestDateRFC3339Valid(t *testing.T) {
	if !dateRFC3339("2025-06-15T12:30:00Z") {
		t.Fatal("valid RFC3339 rejected")
	}
}

func TestDateRFC3339Invalid(t *testing.T) {
	if dateRFC3339("garbage") {
		t.Fatal("invalid RFC3339 accepted")
	}
}

func TestDateRFC1123Valid(t *testing.T) {
	if !dateRFC1123("Mon, 15 Jun 2025 12:30:00 GMT") {
		t.Fatal("valid RFC1123 rejected")
	}
}

func TestDateRFC1123Invalid(t *testing.T) {
	if dateRFC1123("bad") {
		t.Fatal("invalid RFC1123 accepted")
	}
}

func TestDatePlayfabValid(t *testing.T) {
	if !datePlayfab("2025-06-15T12:30:00.000Z") {
		t.Fatal("valid playfab date rejected")
	}
}

func TestDatePlayfabInvalid(t *testing.T) {
	if datePlayfab("nope") {
		t.Fatal("invalid playfab date accepted")
	}
}

func TestCompareJSONIdentical(t *testing.T) {
	a := map[string]any{"key": "value", "num": float64(42)}
	if !CompareJSON(a, a) {
		t.Fatal("identical JSON should compare equal")
	}
}

func TestCompareJSONDifferentNonVolatile(t *testing.T) {
	a := map[string]any{"key": "value1"}
	b := map[string]any{"key": "value2"}
	if CompareJSON(a, b) {
		t.Fatal("different non-volatile values should not match")
	}
}

func TestCompareJSONSameVolatileEpoch(t *testing.T) {
	now := float64(time.Now().Unix())
	a := map[string]any{"ts": now}
	b := map[string]any{"ts": now + 100}
	if !CompareJSON(a, b) {
		t.Fatal("epoch values within range should be treated as volatile")
	}
}

func TestCompareJSONSameVolatileIP(t *testing.T) {
	a := map[string]any{"ip": "192.168.1.1"}
	b := map[string]any{"ip": "10.0.0.1"}
	if !CompareJSON(a, b) {
		t.Fatal("different IPs should be treated as volatile")
	}
}

func TestCompareJSONSameVolatileDate(t *testing.T) {
	a := map[string]any{"date": "2025-06-15T12:30:00.000Z"}
	b := map[string]any{"date": "2025-06-15T12:35:00.000Z"}
	if !CompareJSON(a, b) {
		t.Fatal("different ISO 8601 dates should be treated as volatile")
	}
}
