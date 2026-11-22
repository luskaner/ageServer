package hosts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// A standalone API server means two mapping passes into one file. Each domain
// has to end up on its own server's address, and a revert has to remove both.
func TestApiAndRootPassesShareOneHostsFile(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(fpath, []byte("127.0.0.1 localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}
	apiIP := net.ParseIP("10.0.0.2")
	rootIP := net.ParseIP("10.0.0.3")
	if ok, err := AddApiHosts(apiIP, "age2", fpath, "\n", false, nil); err != nil || !ok {
		t.Fatalf("AddApiHosts: ok=%v err=%v", ok, err)
	}
	if ok, err := AddRootHosts(rootIP, "age2", fpath, "\n", false, nil); err != nil || !ok {
		t.Fatalf("AddRootHosts: ok=%v err=%v", ok, err)
	}

	content, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]net.IP{}
	for line := range strings.SplitSeq(string(content), "\n") {
		ok, _, parsed := ParseLine(line, true)
		if !ok || parsed.OnlyComments() {
			continue
		}
		for _, host := range parsed.Hosts() {
			got[host.String()] = parsed.IP()
		}
	}
	if ip := got[common.ApiAgeOfEmpires]; !ip.Equal(apiIP) {
		t.Errorf("API domain mapped to %v, want %v", ip, apiIP)
	}
	if ip := got[common.CdnAgeOfEmpires]; !ip.Equal(rootIP) {
		t.Errorf("root domain mapped to %v, want %v", ip, rootIP)
	}

	// The revert strips every line this package wrote, whichever pass added it.
	stripped, err := StrippedContents(content)
	if err != nil {
		t.Fatalf("StrippedContents: %v", err)
	}
	for _, domain := range []string{common.ApiAgeOfEmpires, common.CdnAgeOfEmpires} {
		if strings.Contains(string(stripped), domain) {
			t.Fatalf("%q survived the revert: %q", domain, stripped)
		}
	}
	if !strings.Contains(string(stripped), "localhost") {
		t.Fatalf("the revert dropped a line it did not write: %q", stripped)
	}
}
