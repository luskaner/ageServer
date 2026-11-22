package common

import (
	"slices"
	"testing"

	commonGame "github.com/luskaner/ageServer/common/game"
)

// The API domains and the root domains are the two halves of AllHosts: every
// domain has to land in exactly one of them, or a mapping pass would either
// leave a domain unanswered or point it at the wrong server.
func TestApiHostsAndRootHostsPartitionAllHosts(t *testing.T) {
	seedGeneratedDomains(t)
	for _, gameId := range []string{
		commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoE4, commonGame.AoM,
	} {
		t.Run(gameId, func(t *testing.T) {
			root := RootHosts(gameId, false)
			api := ApiHosts(gameId, false)
			union := append(slices.Clone(root), api...)
			if want := AllHosts(gameId, false); !slices.Equal(union, want) {
				t.Fatalf("RootHosts+ApiHosts = %v, want AllHosts = %v", union, want)
			}
			for _, domain := range api {
				if slices.Contains(root, domain) {
					t.Fatalf("%q is in both halves", domain)
				}
			}
		})
	}
}

func TestApiHostsPerGame(t *testing.T) {
	for _, gameId := range []string{commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoM} {
		if got := ApiHosts(gameId, false); !slices.Equal(got, []string{ApiAgeOfEmpires}) {
			t.Errorf("%s API hosts = %v, want %v", gameId, got, []string{ApiAgeOfEmpires})
		}
	}
	if got := ApiHosts(commonGame.AoE4, false); !slices.Equal(got, []string{Aoe4ApiAgeOfEmpires}) {
		t.Errorf("AoE4 API hosts = %v, want %v", got, []string{Aoe4ApiAgeOfEmpires})
	}
}

// The macOS-only domains belong to the root server: the API server is the same
// for every platform.
func TestMacOsDomainsStayOnTheRootSide(t *testing.T) {
	seedGeneratedDomains(t)
	if !slices.Contains(RootHosts(commonGame.AoE2, true), aoe2MacWorldsEdgeDomain) {
		t.Fatal("the root hosts of AoE2 on macOS must include the exclusive domain")
	}
	if slices.Contains(ApiHosts(commonGame.AoE2, true), aoe2MacWorldsEdgeDomain) {
		t.Fatal("the API hosts must not include the macOS-exclusive domain")
	}
}
