package hosts

import (
	"net"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestMappings_NilIP(t *testing.T) {
	m := Mappings(game.AoE2, nil, false)
	if len(m) != 0 {
		t.Errorf("Mappings with nil IP should be empty, got %d entries", len(m))
	}
}

func TestMappings_NonNilIP(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	m := Mappings(game.AoE2, ip, false)
	if len(m) == 0 {
		t.Fatal("Mappings with valid IP should not be empty")
	}
	for host, mappedIP := range m {
		if !mappedIP.Equal(ip) {
			t.Errorf("host %v mapped to %v, want %v", host, mappedIP, ip)
		}
	}
}

func TestMappings_ContainsExpectedHosts(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	m := Mappings(game.AoE2, ip, false)
	if _, ok := m[Host("aoe-api.worldsedgelink.com")]; !ok {
		t.Error("expected aoe-api.worldsedgelink.com in mappings")
	}
}

func TestMappings_MacOsExclusive(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	with := Mappings(game.AoE2, ip, true)
	if _, ok := with[Host("arthurlive-release1-api.worldsedgelink.com")]; !ok {
		t.Error("expected arthurlive-release1-api.worldsedgelink.com in the macOS mappings")
	}
	without := Mappings(game.AoE2, ip, false)
	if _, ok := without[Host("arthurlive-release1-api.worldsedgelink.com")]; ok {
		t.Error("the macOS domains must not be mapped without the flag")
	}
}

// The API and root mappings are disjoint and together cover what Mappings
// covers, which is what lets the two setup passes use one hosts file.
func TestApiAndRootMappingsPartitionAllMappings(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	all := Mappings(game.AoE2, ip, false)
	root := RootMappings(game.AoE2, ip, false)
	api := ApiMappings(game.AoE2, ip, false)
	if len(root)+len(api) != len(all) {
		t.Fatalf("root (%d) + api (%d) != all (%d)", len(root), len(api), len(all))
	}
	for host := range api {
		if _, ok := root[host]; ok {
			t.Fatalf("%v is in both mappings", host)
		}
		if _, ok := all[host]; !ok {
			t.Fatalf("%v is missing from the union", host)
		}
	}
}

func TestApiMappingsNilIP(t *testing.T) {
	if m := ApiMappings(game.AoE2, nil, false); len(m) != 0 {
		t.Fatalf("ApiMappings with nil IP should be empty, got %d entries", len(m))
	}
}

func TestMappings_DifferentGames(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	m1 := Mappings(game.AoE1, ip, false)
	m2 := Mappings(game.AoE2, ip, false)
	if len(m1) == 0 || len(m2) == 0 {
		t.Error("both mappings should be non-empty")
	}
}
