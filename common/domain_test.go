package common

import (
	"errors"
	"slices"
	"strings"
	"testing"

	commonGame "github.com/luskaner/ageServer/common/game"
)

// Seeds the DNS-probe cache so generateDomains never performs network lookups.
func seedGeneratedDomains(tb testing.TB) {
	tb.Helper()
	for _, g := range []string{
		commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoE4, commonGame.AoM,
	} {
		generatedDomainsCache[g] = nil
	}
	tb.Cleanup(func() {
		for _, g := range []string{
			commonGame.AoE1, commonGame.AoE2, commonGame.AoE3, commonGame.AoE4, commonGame.AoM,
		} {
			delete(generatedDomainsCache, g)
		}
	})
}

// Regression for the discarded AoE4 domains: the release-specific hosts must
// survive alongside the shared relic/worlds-edge ones.
func TestGameHostsAoE4Union(t *testing.T) {
	seedGeneratedDomains(t)
	domains := GameHosts(commonGame.AoE4, false)

	for _, want := range []string{
		aoe4SubDomainPrefix + "1" + apiWorldsEdge,
		aoe4SubDomainPrefix + "2" + apiWorldsEdge,
		relicDomain,
		SubDomain + worldsEdge + dotTld,
	} {
		if !slices.Contains(domains, want) {
			t.Errorf("AoE4 domains missing %q (got %v)", want, domains)
		}
	}
	if len(domains) != 4 {
		t.Errorf("AoE4 domains = %v, want exactly 4 entries", domains)
	}
}

func TestGameHostsSharedGames(t *testing.T) {
	seedGeneratedDomains(t)
	want := []string{relicDomain, SubDomain + worldsEdge + dotTld}
	for _, g := range []string{commonGame.AoE1, commonGame.AoE2, commonGame.AoE3} {
		got := GameHosts(g, false)
		if !slices.Equal(got[:2], want) {
			t.Errorf("%s: got %v, want prefix %v", g, got[:2], want)
		}
	}
}

func TestGameHostsMacOsExclusiveOnlyAoE2(t *testing.T) {
	seedGeneratedDomains(t)
	with := GameHosts(commonGame.AoE2, true)
	if !slices.Contains(with, aoe2MacWorldsEdgeDomain) {
		t.Errorf("AoE2 macOS-exclusive domain missing: %v", with)
	}
	for _, g := range []string{commonGame.AoE1, commonGame.AoE3, commonGame.AoE4, commonGame.AoM} {
		if slices.Contains(GameHosts(g, true), aoe2MacWorldsEdgeDomain) {
			t.Errorf("%s must not include the macOS-exclusive domain", g)
		}
	}
}

func TestAllHostsAdditions(t *testing.T) {
	seedGeneratedDomains(t)
	aom := AllHosts(commonGame.AoM, false)
	if !slices.Contains(aom, "c15f9"+playFabSuffix) || !slices.Contains(aom, ApiAgeOfEmpires) || !slices.Contains(aom, CdnAgeOfEmpires) {
		t.Errorf("AoM AllHosts incomplete: %v", aom)
	}
	aoe4 := AllHosts(commonGame.AoE4, false)
	if !slices.Contains(aoe4, "ed603"+playFabSuffix) || !slices.Contains(aoe4, Aoe4ApiAgeOfEmpires) {
		t.Errorf("AoE4 AllHosts incomplete: %v", aoe4)
	}
}

func TestSelfSignedCertDomainsIncludedInCertDomains(t *testing.T) {
	for _, d := range SelfSignedCertDomains {
		if !slices.Contains(CertDomains(), d) {
			t.Errorf("CertDomains missing self-signed domain %q", d)
		}
	}
}

func TestGenerateDomainsDirect(t *testing.T) {
	origFn := directHostToIPFn
	defer func() { directHostToIPFn = origFn }()
	calls := 0
	directHostToIPFn = func(host string) (string, error) {
		calls++
		if calls <= 1 {
			return "1.2.3.4", nil
		}
		return "", errors.New("mock dns error")
	}
	for _, tc := range []struct {
		gameId     string
		wantMin    int
		wantPrefix string
	}{
		{commonGame.AoE2, 2, SubDomainAge2Prefix},
		{commonGame.AoE4, 2, aoe4Marker},
		{commonGame.AoM, 20, "andromeda"},
		{"unknown", 0, ""},
	} {
		t.Run(tc.gameId, func(t *testing.T) {
			delete(generatedDomainsCache, tc.gameId)
			calls = 0
			got := generateDomains(tc.gameId)
			if tc.wantMin == 0 {
				if len(got) != 0 {
					t.Errorf("generateDomains(%q) = %v, want empty", tc.gameId, got)
				}
				return
			}
			if len(got) < tc.wantMin {
				t.Errorf("generateDomains(%q) len=%d, want >=%d", tc.gameId, len(got), tc.wantMin)
			}
			if !strings.Contains(got[0], tc.wantPrefix) {
				t.Errorf("first domain %q should contain prefix %q", got[0], tc.wantPrefix)
			}
			cached := generateDomains(tc.gameId)
			if !slices.Equal(got, cached) {
				t.Errorf("cached mismatch: got %v, cached %v", got, cached)
			}
		})
	}
}

func withUseInternet(tb testing.TB, enabled bool) {
	tb.Helper()
	oldUseInternet := useInternet
	oldCache := generatedDomainsCache
	useInternet = enabled
	generatedDomainsCache = make(map[string][]string)
	tb.Cleanup(func() {
		useInternet = oldUseInternet
		generatedDomainsCache = oldCache
	})
}

func TestGenerateDomainsNoProbeWhenInternetDisabled(t *testing.T) {
	withUseInternet(t, false)
	oldFn := directHostToIPFn
	defer func() { directHostToIPFn = oldFn }()
	calls := 0
	directHostToIPFn = func(host string) (string, error) {
		calls++
		return "1.2.3.4", nil
	}
	for _, tc := range []struct {
		gameId          string
		wantReleaseMin  int
		wantReleasePart string
	}{
		{commonGame.AoE2, 7, stdSubDomainReleasePart},
		{commonGame.AoE4, 10, "-activerelease"},
		{commonGame.AoM, 22, stdSubDomainReleasePart},
	} {
		t.Run(tc.gameId, func(t *testing.T) {
			delete(generatedDomainsCache, tc.gameId)
			calls = 0
			got := generateDomains(tc.gameId)
			if calls != 0 {
				t.Errorf("directHostToIPFn called %d times with internet disabled, want 0", calls)
			}
			if len(got) != tc.wantReleaseMin {
				t.Errorf("generateDomains(%q) len=%d, want %d (only static releases)", tc.gameId, len(got), tc.wantReleaseMin)
			}
			if !strings.Contains(got[len(got)-1], tc.wantReleasePart) {
				t.Errorf("last domain %q should contain %q", got[len(got)-1], tc.wantReleasePart)
			}
		})
	}
}

func TestGenerateDomainsProbesWhenInternetEnabled(t *testing.T) {
	withUseInternet(t, true)
	oldFn := directHostToIPFn
	defer func() { directHostToIPFn = oldFn }()
	calls := 0
	directHostToIPFn = func(host string) (string, error) {
		calls++
		if calls <= 2 {
			return "1.2.3.4", nil
		}
		return "", errors.New("mock dns error")
	}
	delete(generatedDomainsCache, commonGame.AoE2)
	got := generateDomains(commonGame.AoE2)
	if calls != 3 {
		t.Errorf("directHostToIPFn called %d times with internet enabled, want 3", calls)
	}
	if len(got) != 9 {
		t.Errorf("generateDomains(\"age2\") len=%d, want 9 (7 static + 2 probed)", len(got))
	}
}

func TestSetUseInternetInvalidatesCache(t *testing.T) {
	withUseInternet(t, true)
	oldFn := directHostToIPFn
	defer func() { directHostToIPFn = oldFn }()
	calls := 0
	directHostToIPFn = func(host string) (string, error) {
		calls++
		if calls <= 1 {
			return "1.2.3.4", nil
		}
		return "", errors.New("mock dns error")
	}
	SetUseInternet(true)
	if _, cached := generatedDomainsCache[commonGame.AoE2]; cached {
		t.Fatal("cache should be empty immediately after SetUseInternet")
	}
	got := generateDomains(commonGame.AoE2)
	if len(got) != 8 {
		t.Errorf("generateDomains(\"age2\") with internet enabled len=%d, want 8 (7 static + 1 probed)", len(got))
	}
	SetUseInternet(false)
	if _, cached := generatedDomainsCache[commonGame.AoE2]; cached {
		t.Fatal("cache should be invalidated when SetUseInternet switches the setting")
	}
	got = generateDomains(commonGame.AoE2)
	if len(got) != 7 {
		t.Errorf("generateDomains(\"age2\") after disabling internet len=%d, want 7 static releases", len(got))
	}
}
