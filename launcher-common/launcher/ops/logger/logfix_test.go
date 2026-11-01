package logger

import (
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/launcher-common/userData"
)

// Regression: matchPattern used == (case-sensitive), but DNS names are
// case-insensitive per RFC 1035.
func TestMatchPatternCaseInsensitive(t *testing.T) {
	hosts := []string{"AoE2-DE.worldsedgelink.com", "sub.example.COM"}

	if !matchPattern("aoe2-de.worldsedgelink.com", hosts) {
		t.Error("exact match with different case must succeed")
	}
	if !matchPattern("*.worldsedgelink.COM", hosts) {
		t.Error("wildcard suffix with different case must succeed")
	}
	if matchPattern("*.other.com", hosts) {
		t.Error("non-matching domain must not match")
	}
}

func TestMatchPatternExact(t *testing.T) {
	hosts := []string{"example.com"}
	if !matchPattern("example.com", hosts) {
		t.Fatal("exact match must succeed")
	}
	if matchPattern("other.com", hosts) {
		t.Fatal("different host must not match")
	}
}

func TestWriteDataInfoCoversAllTypes(t *testing.T) {
	for _, typ := range []int{userData.TypeActive, userData.TypeServer, userData.TypeBackup} {
		if _, ok := dataTypeToString[typ]; !ok {
			t.Fatalf("dataTypeToString missing type %d", typ)
		}
	}
}

func TestMatchPatternWildcardSubdomainDot(t *testing.T) {
	hosts := []string{"a.b.example.com"}
	if matchPattern("*.example.com", hosts) {
		t.Error("multi-level subdomain prefix (contains dot) should not match")
	}
}

func TestMatchPatternWildcardEmptyPrefix(t *testing.T) {
	hosts := []string{".example.com"}
	if matchPattern("*.example.com", hosts) {
		t.Error("empty prefix should not match")
	}
}

func TestPrintFileNonexistentDoesNotPanic(t *testing.T) {
	PrintFile("test", filepath.Join(t.TempDir(), "nonexistent.txt"))
}
